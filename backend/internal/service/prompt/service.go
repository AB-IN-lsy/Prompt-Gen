package prompt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	promptdomain "electron-go-app/backend/internal/domain/prompt"
	modeldomain "electron-go-app/backend/internal/infra/model/deepseek"
	"electron-go-app/backend/internal/infra/ratelimit"
	"electron-go-app/backend/internal/repository"
	modelsvc "electron-go-app/backend/internal/service/model"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ModelInvoker 抽象模型服务，便于在单元测试中注入假实现。
type ModelInvoker interface {
	InvokeChatCompletion(ctx context.Context, userID uint, modelKey string, req modeldomain.ChatCompletionRequest) (modeldomain.ChatCompletionResponse, error)
}

// WorkspaceStore 抽象 Redis 工作区的读写接口。
type WorkspaceStore interface {
	CreateOrReplace(ctx context.Context, userID uint, snapshot promptdomain.WorkspaceSnapshot) (string, error)
	MergeKeywords(ctx context.Context, userID uint, token string, keywords []promptdomain.WorkspaceKeyword) error
	RemoveKeyword(ctx context.Context, userID uint, token, polarity, word string) error
	UpdateDraftBody(ctx context.Context, userID uint, token, body string) error
	SetAttributes(ctx context.Context, userID uint, token string, attrs map[string]string) error
	Touch(ctx context.Context, userID uint, token string) error
	Snapshot(ctx context.Context, userID uint, token string) (promptdomain.WorkspaceSnapshot, error)
	Delete(ctx context.Context, userID uint, token string) error
	SetPromptMeta(ctx context.Context, userID uint, token string, promptID uint, status string) error
	GetPromptMeta(ctx context.Context, userID uint, token string) (uint, string, error)
}

// PersistenceQueue 描述异步落库队列的最小能力集合。
type PersistenceQueue interface {
	Enqueue(ctx context.Context, task promptdomain.PersistenceTask) (string, error)
	BlockingPop(ctx context.Context, timeout time.Duration) (promptdomain.PersistenceTask, error)
}

// Service 汇总 Prompt 工作台所需的核心能力，包括：
// 1. 解析自然语言描述获取 topic/关键词；
// 2. 基于现有关键词让模型补全缺口；
// 3. 生成 Prompt 正文并保存历史版本；
// 4. 维护关键词字典，支持手动新增。
type Service struct {
	prompts             *repository.PromptRepository
	keywords            *repository.KeywordRepository
	model               ModelInvoker
	auditModel          ModelInvoker
	auditModelKey       string
	workspace           WorkspaceStore
	queue               PersistenceQueue
	logger              *zap.SugaredLogger
	keywordLimit        int
	keywordMaxLength    int
	tagLimit            int
	tagMaxLength        int
	listDefaultPageSize int
	listMaxPageSize     int
	useFullText         bool
	exportDir           string
	versionKeepLimit    int
	importBatchSize     int
	freeTier            *freeTier
}

const (
	defaultQueuePollInterval     = 2 * time.Second
	defaultModelInvokeTimeout    = 35 * time.Second
	defaultWorkspaceWriteTimeout = 5 * time.Second
)

// DefaultKeywordLimit 限制正/负向关键词数量的默认值。
const DefaultKeywordLimit = 10

// DefaultKeywordMaxLength 限制单个关键词的最大字符数（按 rune 计）。
const DefaultKeywordMaxLength = 32

// DefaultTagLimit 限制单个 Prompt 标签数量的默认值。
const DefaultTagLimit = 3

// DefaultTagMaxLength 限制标签字符数，默认 5 个字符。
const DefaultTagMaxLength = 5

// DefaultPromptListPageSize 定义个人 Prompt 列表的默认分页大小。
const DefaultPromptListPageSize = 10

// DefaultPromptListMaxPageSize 定义个人 Prompt 列表的最大分页大小。
const DefaultPromptListMaxPageSize = 100

// DefaultVersionRetentionLimit 默认保留的 Prompt 历史版本数量。
const DefaultVersionRetentionLimit = 5

// DefaultImportBatchSize 控制导入 Prompt 时的批处理大小。
const DefaultImportBatchSize = 20

const (
	workspaceAttrInstructions = "instructions"
	workspaceAttrTags         = "tags"
)

const (
	// minKeywordWeight/maxKeywordWeight 定义关键词相关度（0~5）的允许区间。
	minKeywordWeight = 0
	maxKeywordWeight = 5
	// defaultKeywordWeight 用于 Interpret/Augment 未返回权重或手动录入时的兜底值。
	defaultKeywordWeight = 5
)

const (
	// DefaultPromptExportDir 定义 Prompt 导出的默认保存路径（相对项目根目录）。
	DefaultPromptExportDir = "data/exports"
	// exportDirPermission 指定导出目录的权限。
	exportDirPermission os.FileMode = 0o755
	// exportFilePermission 指定导出文件的权限。
	exportFilePermission os.FileMode = 0o600
)

const (
	likeIncrement = 1
	likeDecrement = -1
)

// auditStage 标记内容审核所处的业务阶段，便于提示模型给出针对性反馈。
type auditStage string

const (
	auditStageInterpretInput auditStage = "interpret_input"
	auditStageGenerateOutput auditStage = "generate_output"
	auditStageCommentBody    auditStage = "comment_body"
)

var (
	// ErrPositiveKeywordLimit 表示正向关键词数量超出上限。
	ErrPositiveKeywordLimit = errors.New("positive keywords exceed limit")
	// ErrNegativeKeywordLimit 表示负向关键词数量超出上限。
	ErrNegativeKeywordLimit = errors.New("negative keywords exceed limit")
	// ErrDuplicateKeyword 表示同极性的关键词已存在。
	ErrDuplicateKeyword = errors.New("keyword already exists")
	// ErrPromptNotFound 表示指定 Prompt 不存在或无访问权限。
	ErrPromptNotFound = errors.New("prompt not found")
	// ErrPromptVersionNotFound 表示请求的历史版本不存在。
	ErrPromptVersionNotFound = errors.New("prompt version not found")
	// ErrContentRejected 表示内容审核未通过，需提醒用户修改。
	ErrContentRejected = errors.New("content rejected")
	// ErrTagLimitExceeded 表示标签数量超出上限。
	ErrTagLimitExceeded = errors.New("tags exceed limit")
	// ErrModelInvocationFailed 表示调用模型失败，通常由网络或凭据问题导致。
	ErrModelInvocationFailed = errors.New("model invocation failed")
)

// Config 汇总 Prompt 服务的可配置参数。
type Config struct {
	KeywordLimit        int
	KeywordMaxLength    int
	TagLimit            int
	TagMaxLength        int
	DefaultListPageSize int
	MaxListPageSize     int
	UseFullTextSearch   bool
	ExportDirectory     string
	VersionRetention    int
	ImportBatchSize     int
	Audit               AuditConfig
	FreeTier            FreeTierConfig
}

// FreeTierUsageSnapshot 描述免费额度的当前余量与重置时间。
type FreeTierUsageSnapshot struct {
	Limit      int
	Remaining  int
	ResetAfter time.Duration
}

// NewServiceWithConfig 构建 Service，并允许自定义分页等配置。
func NewServiceWithConfig(prompts *repository.PromptRepository, keywords *repository.KeywordRepository, model ModelInvoker, workspace WorkspaceStore, queue PersistenceQueue, logger *zap.SugaredLogger, freeLimiter ratelimit.Limiter, cfg Config) (*Service, error) {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	if cfg.KeywordLimit <= 0 {
		cfg.KeywordLimit = DefaultKeywordLimit
	}
	if cfg.KeywordMaxLength <= 0 {
		cfg.KeywordMaxLength = DefaultKeywordMaxLength
	}
	if cfg.TagLimit <= 0 {
		cfg.TagLimit = DefaultTagLimit
	}
	if cfg.TagMaxLength <= 0 {
		cfg.TagMaxLength = DefaultTagMaxLength
	}
	if cfg.DefaultListPageSize <= 0 {
		cfg.DefaultListPageSize = DefaultPromptListPageSize
	}
	if cfg.MaxListPageSize <= 0 {
		cfg.MaxListPageSize = DefaultPromptListMaxPageSize
	}
	if cfg.DefaultListPageSize > cfg.MaxListPageSize {
		cfg.DefaultListPageSize = cfg.MaxListPageSize
	}
	if cfg.VersionRetention <= 0 {
		cfg.VersionRetention = DefaultVersionRetentionLimit
	}
	if cfg.ImportBatchSize <= 0 {
		cfg.ImportBatchSize = DefaultImportBatchSize
	}
	baseExportDir := strings.TrimSpace(cfg.ExportDirectory)
	if baseExportDir == "" {
		baseExportDir = DefaultPromptExportDir
	}
	normalisedExportDir, err := normaliseExportDir(baseExportDir)
	if err != nil {
		logger.Warnw("normalize prompt export directory failed, fallback to default", "path", baseExportDir, "error", err)
		if fallback, fallbackErr := normaliseExportDir(DefaultPromptExportDir); fallbackErr == nil {
			normalisedExportDir = fallback
		} else {
			normalisedExportDir = filepath.Clean(DefaultPromptExportDir)
			logger.Warnw("fallback prompt export directory normalization failed", "error", fallbackErr)
		}
	}
	auditInvoker, auditModelKey, err := buildAuditInvoker(cfg.Audit)
	if err != nil {
		return nil, fmt.Errorf("build audit invoker: %w", err)
	}

	freeTier, err := buildFreeTier(cfg.FreeTier, freeLimiter, logger)
	if err != nil {
		return nil, fmt.Errorf("build free tier: %w", err)
	}

	return &Service{
		prompts:             prompts,
		keywords:            keywords,
		model:               model,
		auditModel:          auditInvoker,
		auditModelKey:       strings.TrimSpace(auditModelKey),
		workspace:           workspace,
		queue:               queue,
		logger:              logger,
		keywordLimit:        cfg.KeywordLimit,
		keywordMaxLength:    cfg.KeywordMaxLength,
		tagLimit:            cfg.TagLimit,
		tagMaxLength:        cfg.TagMaxLength,
		listDefaultPageSize: cfg.DefaultListPageSize,
		listMaxPageSize:     cfg.MaxListPageSize,
		useFullText:         cfg.UseFullTextSearch,
		exportDir:           normalisedExportDir,
		versionKeepLimit:    cfg.VersionRetention,
		importBatchSize:     cfg.ImportBatchSize,
		freeTier:            freeTier,
	}, nil
}

// KeywordLimit 暴露当前生效的关键词数量上限，供上层 Handler 使用。
func (s *Service) KeywordLimit() int {
	return s.keywordLimit
}

// TagLimit 返回标签数量限制，供 handler 构造提示信息。
func (s *Service) TagLimit() int {
	return s.tagLimit
}

// ListPageSizeDefaults 返回列表分页的默认与最大页大小。
func (s *Service) ListPageSizeDefaults() (int, int) {
	return s.listDefaultPageSize, s.listMaxPageSize
}

// FreeTierInfo 返回免费额度模型的基础描述信息，供上层聚合展示。
func (s *Service) FreeTierInfo() *FreeTierInfo {
	if s == nil || s.freeTier == nil {
		return nil
	}
	return s.freeTier.info()
}

// FreeTierUsage 返回指定用户的免费额度剩余情况。
func (s *Service) FreeTierUsage(ctx context.Context, userID uint) (*FreeTierUsageSnapshot, error) {
	if s == nil || s.freeTier == nil || !s.freeTier.enabled {
		return nil, nil
	}
	if userID == 0 {
		return nil, errors.New("user id required")
	}
	usage, ttl, err := s.freeTier.snapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &FreeTierUsageSnapshot{
		Limit:      usage.Limit,
		Remaining:  usage.Remaining,
		ResetAfter: ttl,
	}, nil
}

// ListPrompts 返回当前用户的 Prompt 列表，支持状态筛选与关键字搜索。
func (s *Service) ListPrompts(ctx context.Context, input ListPromptsInput) (ListPromptsOutput, error) {
	page := input.Page
	if page <= 0 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = s.listDefaultPageSize
	}
	if pageSize > s.listMaxPageSize {
		pageSize = s.listMaxPageSize
	}
	filter := repository.PromptListFilter{
		Status:      strings.TrimSpace(input.Status),
		Query:       strings.TrimSpace(input.Query),
		UseFullText: s.useFullText,
		Limit:       pageSize,
		Offset:      (page - 1) * pageSize,
		Favorited:   input.FavoritedOnly,
	}
	records, total, err := s.prompts.ListByUser(ctx, input.UserID, filter)
	if err != nil {
		return ListPromptsOutput{}, err
	}
	items := make([]PromptSummary, 0, len(records))
	for _, record := range records {
		positive := s.clampKeywordList(decodePromptKeywords(record.PositiveKeywords))
		negative := s.clampKeywordList(decodePromptKeywords(record.NegativeKeywords))
		items = append(items, PromptSummary{
			ID:               record.ID,
			Topic:            record.Topic,
			Model:            record.Model,
			Status:           record.Status,
			Tags:             s.truncateTags(decodeTags(record.Tags)),
			PositiveKeywords: positive,
			NegativeKeywords: negative,
			IsFavorited:      record.IsFavorited,
			IsLiked:          record.IsLiked,
			LikeCount:        record.LikeCount,
			UpdatedAt:        record.UpdatedAt,
			PublishedAt:      record.PublishedAt,
		})
	}
	return ListPromptsOutput{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ListPromptVersions 列出指定 Prompt 的历史版本概览，便于用户选择回溯。
func (s *Service) ListPromptVersions(ctx context.Context, input ListVersionsInput) (ListVersionsOutput, error) {
	if input.UserID == 0 || input.PromptID == 0 {
		return ListVersionsOutput{}, errors.New("user id and prompt id are required")
	}
	if _, err := s.prompts.FindByID(ctx, input.UserID, input.PromptID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ListVersionsOutput{}, ErrPromptNotFound
		}
		return ListVersionsOutput{}, err
	}
	limit := input.Limit
	if limit <= 0 {
		limit = s.versionKeepLimit
	}
	versions, err := s.prompts.ListVersions(ctx, input.PromptID, limit)
	if err != nil {
		return ListVersionsOutput{}, err
	}
	summaries := make([]PromptVersionSummary, 0, len(versions))
	for _, version := range versions {
		summaries = append(summaries, PromptVersionSummary{
			VersionNo: version.VersionNo,
			Model:     version.Model,
			CreatedAt: version.CreatedAt,
		})
	}
	return ListVersionsOutput{Versions: summaries}, nil
}

// GetPromptVersionDetail 返回指定 Prompt 版本的完整内容，用于历史查看与回溯。
func (s *Service) GetPromptVersionDetail(ctx context.Context, input GetVersionDetailInput) (PromptVersionDetail, error) {
	if input.UserID == 0 || input.PromptID == 0 || input.VersionNo <= 0 {
		return PromptVersionDetail{}, errors.New("invalid version query parameters")
	}
	if _, err := s.prompts.FindByID(ctx, input.UserID, input.PromptID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PromptVersionDetail{}, ErrPromptNotFound
		}
		return PromptVersionDetail{}, err
	}
	version, err := s.prompts.FindVersion(ctx, input.PromptID, input.VersionNo)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PromptVersionDetail{}, ErrPromptVersionNotFound
		}
		return PromptVersionDetail{}, err
	}
	positive := s.clampKeywordList(decodePromptKeywords(version.PositiveKeywords))
	negative := s.clampKeywordList(decodePromptKeywords(version.NegativeKeywords))
	return PromptVersionDetail{
		VersionNo:        version.VersionNo,
		Body:             version.Body,
		Instructions:     version.Instructions,
		Model:            version.Model,
		PositiveKeywords: positive,
		NegativeKeywords: negative,
		CreatedAt:        version.CreatedAt,
	}, nil
}

// ExportPromptsInput 描述导出 Prompt 时所需的参数。
type ExportPromptsInput struct {
	UserID uint
}

// ExportPromptsOutput 返回导出文件的路径与摘要数据。
type ExportPromptsOutput struct {
	FilePath    string
	PromptCount int
	GeneratedAt time.Time
}

type promptExportRecord struct {
	ID               uint                             `json:"id"`
	Topic            string                           `json:"topic"`
	Body             string                           `json:"body"`
	Instructions     string                           `json:"instructions"`
	Model            string                           `json:"model"`
	Status           string                           `json:"status"`
	Tags             []string                         `json:"tags"`
	PositiveKeywords []promptdomain.PromptKeywordItem `json:"positive_keywords"`
	NegativeKeywords []promptdomain.PromptKeywordItem `json:"negative_keywords"`
	IsFavorited      bool                             `json:"is_favorited"`
	PublishedAt      *time.Time                       `json:"published_at,omitempty"`
	CreatedAt        time.Time                        `json:"created_at"`
	UpdatedAt        time.Time                        `json:"updated_at"`
	LatestVersionNo  int                              `json:"latest_version_no"`
}

type promptExportEnvelope struct {
	GeneratedAt time.Time            `json:"generated_at"`
	PromptCount int                  `json:"prompt_count"`
	Prompts     []promptExportRecord `json:"prompts"`
}

// ImportMode 定义导入 Prompt 时的模式。
type ImportMode string

const (
	importModeMerge     ImportMode = "merge"
	importModeOverwrite ImportMode = "overwrite"
)

// ImportPromptsInput 描述导入 Prompt 时的请求参数。
type ImportPromptsInput struct {
	UserID  uint
	Mode    string
	Payload []byte
}

// ImportPromptsResult 返回导入过程中的统计信息。
type ImportPromptsResult struct {
	Imported int
	Skipped  int
	Errors   []ImportError
}

// ImportError 用于记录导入失败的 Prompt 以及失败原因。
type ImportError struct {
	Topic  string `json:"topic"`
	Reason string `json:"reason"`
}

// ExportPrompts 将用户的 Prompt 序列化为本地 JSON 文件并返回保存结果。
func (s *Service) ExportPrompts(ctx context.Context, input ExportPromptsInput) (ExportPromptsOutput, error) {
	if input.UserID == 0 {
		return ExportPromptsOutput{}, errors.New("user id is required")
	}

	records, _, err := s.prompts.ListByUser(ctx, input.UserID, repository.PromptListFilter{})
	if err != nil {
		return ExportPromptsOutput{}, fmt.Errorf("list prompts for export: %w", err)
	}

	exportItems := make([]promptExportRecord, 0, len(records))
	for _, record := range records {
		positive := keywordItemsToDomain(decodePromptKeywords(record.PositiveKeywords))
		negative := keywordItemsToDomain(decodePromptKeywords(record.NegativeKeywords))
		exportItems = append(exportItems, promptExportRecord{
			ID:               record.ID,
			Topic:            record.Topic,
			Body:             record.Body,
			Instructions:     record.Instructions,
			Model:            record.Model,
			Status:           record.Status,
			Tags:             decodeTags(record.Tags),
			PositiveKeywords: positive,
			NegativeKeywords: negative,
			IsFavorited:      record.IsFavorited,
			PublishedAt:      record.PublishedAt,
			CreatedAt:        record.CreatedAt,
			UpdatedAt:        record.UpdatedAt,
			LatestVersionNo:  record.LatestVersionNo,
		})
	}

	payload := promptExportEnvelope{
		GeneratedAt: time.Now().UTC(),
		PromptCount: len(exportItems),
		Prompts:     exportItems,
	}

	if err := os.MkdirAll(s.exportDir, exportDirPermission); err != nil {
		return ExportPromptsOutput{}, fmt.Errorf("create export directory: %w", err)
	}

	fileName := fmt.Sprintf("prompt-export-%s-%d.json", payload.GeneratedAt.Format("20060102-150405"), input.UserID)
	filePath := filepath.Join(s.exportDir, fileName)

	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return ExportPromptsOutput{}, fmt.Errorf("marshal export payload: %w", err)
	}

	if err := os.WriteFile(filePath, encoded, exportFilePermission); err != nil {
		return ExportPromptsOutput{}, fmt.Errorf("write export file: %w", err)
	}

	return ExportPromptsOutput{
		FilePath:    filePath,
		PromptCount: len(exportItems),
		GeneratedAt: payload.GeneratedAt,
	}, nil
}

// ImportPrompts 根据导出的 JSON 文件内容回灌 Prompt 数据，支持合并或覆盖模式。
func (s *Service) ImportPrompts(ctx context.Context, input ImportPromptsInput) (ImportPromptsResult, error) {
	var result ImportPromptsResult
	if input.UserID == 0 {
		return result, errors.New("user id is required")
	}
	payload := bytes.TrimSpace(input.Payload)
	if len(payload) == 0 {
		return result, errors.New("import payload is empty")
	}
	var envelope promptExportEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return result, fmt.Errorf("decode import payload: %w", err)
	}
	mode := normaliseImportMode(input.Mode)
	if mode == importModeOverwrite {
		if err := s.prompts.DeleteByUser(ctx, input.UserID); err != nil {
			return result, fmt.Errorf("clear prompts before import: %w", err)
		}
	}
	batchSize := s.importBatchSize
	if batchSize <= 0 {
		batchSize = DefaultImportBatchSize
	}
	total := len(envelope.Prompts)
	for idx, record := range envelope.Prompts {
		if err := s.importPromptRecord(ctx, input.UserID, record); err != nil {
			result.Skipped++
			topic := strings.TrimSpace(record.Topic)
			if topic == "" {
				topic = fmt.Sprintf("prompt#%d", idx+1)
			}
			result.Errors = append(result.Errors, ImportError{
				Topic:  topic,
				Reason: err.Error(),
			})
			continue
		}
		result.Imported++
		if batchSize > 0 && (idx+1)%batchSize == 0 {
			s.logger.Infow("prompt import progress", "processed", idx+1, "total", total)
		}
	}
	return result, nil
}

// normaliseImportMode 将导入模式字符串归一化。
func normaliseImportMode(value string) ImportMode {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(importModeOverwrite):
		return importModeOverwrite
	default:
		return importModeMerge
	}
}

// importPromptRecord 将导出的单条 Prompt 记录写回数据库。
func (s *Service) importPromptRecord(ctx context.Context, userID uint, record promptExportRecord) error {
	topic := strings.TrimSpace(record.Topic)
	if topic == "" {
		return errors.New("topic is required")
	}
	if strings.TrimSpace(record.Body) == "" {
		return errors.New("body is required")
	}
	positiveItems := exportKeywordsToKeywordItems(record.PositiveKeywords, promptdomain.KeywordPolarityPositive)
	negativeItems := exportKeywordsToKeywordItems(record.NegativeKeywords, promptdomain.KeywordPolarityNegative)
	status := normalizeStatus(record.Status, record.Status == promptdomain.PromptStatusPublished)

	var promptID uint
	if record.ID > 0 {
		if existing, err := s.prompts.FindByID(ctx, userID, record.ID); err == nil {
			promptID = existing.ID
		}
	}
	if promptID == 0 {
		if existing, err := s.prompts.FindByUserAndTopic(ctx, userID, normalizeMixedLanguageSpacing(topic)); err == nil {
			promptID = existing.ID
		}
	}

	input := SaveInput{
		UserID:           userID,
		PromptID:         promptID,
		Topic:            record.Topic,
		Body:             record.Body,
		Instructions:     record.Instructions,
		Model:            record.Model,
		Status:           status,
		Publish:          status == promptdomain.PromptStatusPublished,
		Tags:             record.Tags,
		PositiveKeywords: positiveItems,
		NegativeKeywords: negativeItems,
	}

	result, err := s.persistPrompt(ctx, input, status, "")
	if err != nil {
		return err
	}

	if err := s.prompts.UpdateFavorite(ctx, userID, result.PromptID, record.IsFavorited); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("sync favorite flag: %w", err)
		}
	}

	if err := s.syncImportedMetadata(ctx, result.PromptID, record, status); err != nil {
		return err
	}
	return nil
}

// exportKeywordsToKeywordItems 将导出的关键词列表转换为内部使用的结构。
func exportKeywordsToKeywordItems(items []promptdomain.PromptKeywordItem, polarity string) []KeywordItem {
	if len(items) == 0 {
		return []KeywordItem{}
	}
	result := make([]KeywordItem, 0, len(items))
	for _, item := range items {
		result = append(result, KeywordItem{
			KeywordID: item.KeywordID,
			Word:      item.Word,
			Source:    item.Source,
			Polarity:  polarity,
			Weight:    clampWeight(item.Weight),
		})
	}
	return result
}

// syncImportedMetadata 会在导入成功后同步时间戳、版本号等附加信息。
func (s *Service) syncImportedMetadata(ctx context.Context, promptID uint, record promptExportRecord, status string) error {
	if promptID == 0 {
		return errors.New("prompt id missing after import")
	}
	meta := repository.PromptMetadataUpdate{
		UsePublishedAt: true,
		PublishedAt:    record.PublishedAt,
	}
	if record.LatestVersionNo > 0 {
		meta.LatestVersionNo = &record.LatestVersionNo
	}
	if !record.CreatedAt.IsZero() {
		meta.CreatedAt = &record.CreatedAt
	}
	if !record.UpdatedAt.IsZero() {
		meta.UpdatedAt = &record.UpdatedAt
	}
	if err := s.prompts.UpdateMetadata(ctx, promptID, meta); err != nil {
		return fmt.Errorf("update prompt metadata: %w", err)
	}
	if status != promptdomain.PromptStatusPublished {
		if err := s.prompts.ReplacePromptVersions(ctx, promptID, nil); err != nil {
			return fmt.Errorf("reset prompt versions: %w", err)
		}
		return nil
	}
	versionNo := record.LatestVersionNo
	if versionNo <= 0 {
		versionNo = 1
	}
	positiveBytes, err := json.Marshal(record.PositiveKeywords)
	if err != nil {
		return fmt.Errorf("encode positive keywords for version: %w", err)
	}
	negativeBytes, err := json.Marshal(record.NegativeKeywords)
	if err != nil {
		return fmt.Errorf("encode negative keywords for version: %w", err)
	}
	version := promptdomain.PromptVersion{
		PromptID:         promptID,
		VersionNo:        versionNo,
		Body:             record.Body,
		Instructions:     record.Instructions,
		PositiveKeywords: string(positiveBytes),
		NegativeKeywords: string(negativeBytes),
		Model:            record.Model,
	}
	if !record.UpdatedAt.IsZero() {
		version.CreatedAt = record.UpdatedAt
	} else if record.PublishedAt != nil {
		version.CreatedAt = *record.PublishedAt
	} else {
		version.CreatedAt = time.Now()
	}
	if err := s.prompts.ReplacePromptVersions(ctx, promptID, []promptdomain.PromptVersion{version}); err != nil {
		return fmt.Errorf("replace prompt versions: %w", err)
	}
	return nil
}

// GetPrompt 根据 ID 返回 Prompt 详情，并尝试为工作台创建 Redis 工作区快照（包含标签）。
func (s *Service) GetPrompt(ctx context.Context, input GetPromptInput) (PromptDetail, error) {
	entity, err := s.prompts.FindByID(ctx, input.UserID, input.PromptID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PromptDetail{}, ErrPromptNotFound
		}
		return PromptDetail{}, err
	}
	detail := PromptDetail{
		ID:               entity.ID,
		Topic:            entity.Topic,
		Body:             entity.Body,
		Instructions:     entity.Instructions,
		Model:            entity.Model,
		Status:           entity.Status,
		Tags:             s.truncateTags(decodeTags(entity.Tags)),
		PositiveKeywords: s.clampKeywordList(decodePromptKeywords(entity.PositiveKeywords)),
		NegativeKeywords: s.clampKeywordList(decodePromptKeywords(entity.NegativeKeywords)),
		IsFavorited:      entity.IsFavorited,
		IsLiked:          entity.IsLiked,
		LikeCount:        entity.LikeCount,
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
		PublishedAt:      entity.PublishedAt,
	}

	if s.workspace != nil {
		snapshot := promptdomain.WorkspaceSnapshot{
			UserID:    input.UserID,
			Topic:     entity.Topic,
			ModelKey:  entity.Model,
			DraftBody: entity.Body,
			Positive:  toWorkspaceKeywords(detail.PositiveKeywords, s.keywordLimit),
			Negative:  toWorkspaceKeywords(detail.NegativeKeywords, s.keywordLimit),
			PromptID:  entity.ID,
			Status:    entity.Status,
			UpdatedAt: time.Now(),
		}
		if tags := detail.Tags; len(tags) > 0 {
			encoded := encodeTagsAttribute(tags)
			snapshot.Attributes = map[string]string{workspaceAttrTags: encoded}
		}
		storeCtx, cancel := s.workspaceContext(ctx)
		token, workspaceErr := s.workspace.CreateOrReplace(storeCtx, input.UserID, snapshot)
		cancel()
		if workspaceErr != nil {
			s.logger.Warnw(
				"create workspace snapshot failed",
				"prompt_id", entity.ID,
				"user_id", input.UserID,
				"error", workspaceErr,
			)
		} else {
			detail.WorkspaceToken = token
		}
	}
	return detail, nil
}

// DeletePrompt 删除 Prompt 及其关联的关键词/版本记录。
func (s *Service) DeletePrompt(ctx context.Context, input DeletePromptInput) error {
	if err := s.prompts.Delete(ctx, input.UserID, input.PromptID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPromptNotFound
		}
		return err
	}
	return nil
}

// UpdateFavorite 用于收藏或取消收藏 Prompt。
func (s *Service) UpdateFavorite(ctx context.Context, input UpdateFavoriteInput) error {
	if input.UserID == 0 || input.PromptID == 0 {
		return errors.New("user id and prompt id are required")
	}
	if err := s.prompts.UpdateFavorite(ctx, input.UserID, input.PromptID, input.Favorited); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPromptNotFound
		}
		return err
	}
	return nil
}

// LikePrompt 处理用户对 Prompt 的点赞操作。
func (s *Service) LikePrompt(ctx context.Context, input UpdateLikeInput) (UpdateLikeOutput, error) {
	return s.changePromptLike(ctx, input, true)
}

// UnlikePrompt 处理用户对 Prompt 的取消点赞操作。
func (s *Service) UnlikePrompt(ctx context.Context, input UpdateLikeInput) (UpdateLikeOutput, error) {
	return s.changePromptLike(ctx, input, false)
}

// changePromptLike 根据动作新增或移除点赞关系，并返回变更后的计数。
func (s *Service) changePromptLike(ctx context.Context, input UpdateLikeInput, like bool) (UpdateLikeOutput, error) {
	if input.UserID == 0 || input.PromptID == 0 {
		return UpdateLikeOutput{}, errors.New("user id and prompt id are required")
	}
	if _, err := s.prompts.FindByID(ctx, input.UserID, input.PromptID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return UpdateLikeOutput{}, ErrPromptNotFound
		}
		return UpdateLikeOutput{}, err
	}

	var delta int
	if like {
		created, err := s.prompts.AddLike(ctx, input.PromptID, input.UserID)
		if err != nil {
			return UpdateLikeOutput{}, fmt.Errorf("add prompt like: %w", err)
		}
		if created {
			delta = likeIncrement
		}
	} else {
		removed, err := s.prompts.RemoveLike(ctx, input.PromptID, input.UserID)
		if err != nil {
			return UpdateLikeOutput{}, fmt.Errorf("remove prompt like: %w", err)
		}
		if removed {
			delta = likeDecrement
		}
	}

	if delta != 0 {
		if err := s.prompts.IncrementLikeCount(ctx, input.PromptID, delta); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return UpdateLikeOutput{}, ErrPromptNotFound
			}
			return UpdateLikeOutput{}, fmt.Errorf("update like count: %w", err)
		}
	}

	refreshed, err := s.prompts.FindByID(ctx, input.UserID, input.PromptID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return UpdateLikeOutput{}, ErrPromptNotFound
		}
		return UpdateLikeOutput{}, err
	}

	return UpdateLikeOutput{
		LikeCount: refreshed.LikeCount,
		Liked:     refreshed.IsLiked,
	}, nil
}

// StartPersistenceWorker 启动后台协程消费 Redis 队列并写入 MySQL。
// 使用 BLPop 轮询 Redis List，将排队的保存任务转为同步落库操作；当未启用 Redis/队列时保持旧行为。
func (s *Service) StartPersistenceWorker(ctx context.Context, pollTimeout time.Duration) {
	if s.queue == nil || s.workspace == nil {
		s.logger.Infow("persistence worker disabled (queue or workspace missing)")
		return
	}
	if pollTimeout <= 0 {
		pollTimeout = defaultQueuePollInterval
	}
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			task, err := s.queue.BlockingPop(ctx, pollTimeout)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, redis.Nil) {
					continue
				}
				if ctx.Err() != nil {
					return
				}
				s.logger.Warnw("dequeue persistence task failed", "error", err)
				continue
			}
			if err := s.processPersistenceTask(ctx, task); err != nil {
				s.logger.Errorw("process persistence task failed", "task_id", task.TaskID, "prompt_id", task.PromptID, "user_id", task.UserID, "error", err)
			}
		}
	}()
}

// modelInvocationContext 在调用外部模型时拆解 HTTP 请求上下文，并为长耗时请求设置安全超时。
// Gin 在响应写入后会取消 request.Context()，直接复用会导致还在进行中的模型调用被中断。
// 这里改用 context.WithoutCancel 继承 Value/Deadline，再包裹一个 35s 超时，确保模型请求能顺利完成。
func (s *Service) modelInvocationContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		return context.WithTimeout(context.Background(), defaultModelInvokeTimeout)
	}
	if deadline, ok := parent.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 && remaining <= defaultModelInvokeTimeout {
			return parent, func() {}
		}
	}
	base := context.WithoutCancel(parent)
	ctx, cancel := context.WithTimeout(base, defaultModelInvokeTimeout)
	return ctx, cancel
}

// workspaceContext 给 Redis 写入操作准备一个不受请求取消影响的上下文，并附带 5 秒兜底超时。
func (s *Service) workspaceContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		return context.WithTimeout(context.Background(), defaultWorkspaceWriteTimeout)
	}
	// Redis 写入也不能依赖 request.Context()，否则页面返回后工作区操作会被取消。
	// 这里与模型调用类似，先去掉 cancel，再附加一个较短的 5s 超时，确保写入及时释放资源。
	base := context.WithoutCancel(parent)
	return context.WithTimeout(base, defaultWorkspaceWriteTimeout)
}

// KeywordItem 表示返回给前端的关键词结构。
type KeywordItem struct {
	KeywordID uint   `json:"keyword_id,omitempty"`
	Word      string `json:"word"`
	Source    string `json:"source"`
	Polarity  string `json:"polarity"`
	Weight    int    `json:"weight"`
}

// ListPromptsInput 定义“我的 Prompt”列表请求参数。
type ListPromptsInput struct {
	UserID        uint
	Status        string
	Query         string
	Page          int
	PageSize      int
	FavoritedOnly bool
}

// ListVersionsInput 描述查询 Prompt 历史版本所需的参数。
type ListVersionsInput struct {
	UserID   uint
	PromptID uint
	Limit    int
}

// PromptSummary 返回给前端的 Prompt 概览信息。
type PromptSummary struct {
	ID               uint
	Topic            string
	Model            string
	Status           string
	Tags             []string
	PositiveKeywords []KeywordItem
	NegativeKeywords []KeywordItem
	IsFavorited      bool
	IsLiked          bool
	LikeCount        uint
	UpdatedAt        time.Time
	PublishedAt      *time.Time
}

// ListPromptsOutput 携带分页后的 Prompt 列表。
type ListPromptsOutput struct {
	Items    []PromptSummary
	Total    int64
	Page     int
	PageSize int
}

// PromptVersionSummary 返回版本列表中的概要信息。
type PromptVersionSummary struct {
	VersionNo int
	Model     string
	CreatedAt time.Time
}

// ListVersionsOutput 携带 Prompt 历史版本的集合。
type ListVersionsOutput struct {
	Versions []PromptVersionSummary
}

// GetPromptInput 描述查询单条 Prompt 详情所需参数。
type GetPromptInput struct {
	UserID   uint
	PromptID uint
}

// GetVersionDetailInput 描述获取指定版本内容的参数。
type GetVersionDetailInput struct {
	UserID    uint
	PromptID  uint
	VersionNo int
}

// PromptDetail 为工作台回填准备的完整 Prompt 信息。
type PromptDetail struct {
	ID               uint
	Topic            string
	Body             string
	Instructions     string
	Model            string
	Status           string
	Tags             []string
	PositiveKeywords []KeywordItem
	NegativeKeywords []KeywordItem
	IsFavorited      bool
	IsLiked          bool
	LikeCount        uint
	WorkspaceToken   string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	PublishedAt      *time.Time
}

// PromptVersionDetail 包含历史版本的完整内容。
type PromptVersionDetail struct {
	VersionNo        int
	Body             string
	Instructions     string
	Model            string
	PositiveKeywords []KeywordItem
	NegativeKeywords []KeywordItem
	CreatedAt        time.Time
}

// DeletePromptInput 描述删除 Prompt 所需参数。
type DeletePromptInput struct {
	UserID   uint
	PromptID uint
}

// UpdateFavoriteInput 描述收藏或取消收藏 Prompt 时的请求参数。
type UpdateFavoriteInput struct {
	UserID    uint
	PromptID  uint
	Favorited bool
}

// UpdateLikeInput 描述点赞或取消点赞 Prompt 时的请求参数。
type UpdateLikeInput struct {
	UserID   uint
	PromptID uint
}

// UpdateLikeOutput 返回点赞状态变化后的关键数据。
type UpdateLikeOutput struct {
	LikeCount uint `json:"like_count"`
	Liked     bool `json:"liked"`
}

// InterpretInput 描述解析自然语言所需的参数。
type InterpretInput struct {
	UserID      uint
	Description string
	ModelKey    string
	Language    string
}

// InterpretOutput 返回结构化的 Topic 与关键词列表。
type InterpretOutput struct {
	Topic            string
	PositiveKeywords []KeywordItem
	NegativeKeywords []KeywordItem
	Confidence       float64
	WorkspaceToken   string
	Instructions     string
	Tags             []string
}

// AugmentInput 描述补充关键词的请求参数。
type AugmentInput struct {
	UserID            uint
	Topic             string
	ModelKey          string
	ExistingPositive  []KeywordItem
	ExistingNegative  []KeywordItem
	WorkspaceToken    string
	Language          string
	RequestedPositive int
	RequestedNegative int
}

// AugmentOutput 返回模型补充后的关键词列表（仅新增部分）。
type AugmentOutput struct {
	Positive []KeywordItem
	Negative []KeywordItem
}

// ManualKeywordInput 描述手动新增关键词时的参数。
type ManualKeywordInput struct {
	UserID         uint
	Topic          string
	Word           string
	Polarity       string
	Source         string
	Language       string
	PromptID       uint
	WorkspaceToken string // interpret 返回的 Redis 工作区 token，用于把手动关键词写入缓存。
	Weight         int
}

// RemoveKeywordInput 描述移除临时工作区关键词所需的参数。
type RemoveKeywordInput struct {
	UserID         uint
	Word           string
	Polarity       string
	WorkspaceToken string
}

// SyncWorkspaceInput 用于将前端排序/权重调整同步到 Redis 工作区。
type SyncWorkspaceInput struct {
	UserID         uint
	WorkspaceToken string
	Positive       []KeywordItem
	Negative       []KeywordItem
}

// GenerateInput 描述生成 Prompt 正文所需的上下文。
//   - interpret 返回的 token 对应 Redis 里 prompt:workspace:{user}:{token} 这一套 Hash/ZSET。后续 augment/manual/generate/save 都带上它，让所有编辑操作只命中
//     Redis，不直接写 MySQL，并能定位到同一个工作区快照。
//   - 前端保存草稿/发布时也把 token 携带上，后端根据 token 读取完整快照，组装并入队后台持久化任务，保存成功后会清理掉这个工作区。
type GenerateInput struct {
	UserID            uint
	Topic             string
	ModelKey          string
	Description       string
	ExistingBody      string
	PositiveKeywords  []KeywordItem
	NegativeKeywords  []KeywordItem
	WorkspaceToken    string // 前端和后端约定的“临时工作区”标识
	Instructions      string
	Tone              string
	Language          string
	Temperature       float64
	MaxTokens         int
	PromptID          uint
	IncludeKeywordRef bool
}

// GenerateOutput 返回生成的 Prompt、模型信息与耗时。
type GenerateOutput struct {
	Model        string
	Prompt       string
	Duration     time.Duration
	Usage        *modeldomain.ChatCompletionUsage
	PositiveUsed []KeywordItem
	NegativeUsed []KeywordItem
}

// auditContent 使用用户选择的模型对文本进行内容审核，审核不通过时返回 ErrContentRejected。
func (s *Service) auditContent(ctx context.Context, userID uint, modelKey string, text string, stage auditStage) error {
	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		return nil
	}
	invoker := s.model
	invokeModelKey := strings.TrimSpace(modelKey)
	if s.auditModel != nil {
		invoker = s.auditModel
		if key := strings.TrimSpace(s.auditModelKey); key != "" {
			invokeModelKey = key
		}
	}
	if strings.TrimSpace(invokeModelKey) == "" {
		return nil
	}
	req := buildAuditRequest(stage, trimmedText)
	req.Model = invokeModelKey
	modelCtx, cancel := s.modelInvocationContext(ctx)
	defer cancel()
	resp, err := invoker.InvokeChatCompletion(modelCtx, userID, invokeModelKey, req)
	if err != nil {
		return fmt.Errorf("content audit failed: %w", err)
	}
	verdict, err := parseAuditPayload(resp)
	if err != nil {
		return err
	}
	if !verdict.Allowed {
		reason := normalizeAuditReason(verdict.Reason)
		return fmt.Errorf("%w: %s", ErrContentRejected, reason)
	}
	return nil
}

// AuditCommentContent 对评论文本执行内容审核。
func (s *Service) AuditCommentContent(ctx context.Context, userID uint, content string) error {
	return s.auditContent(ctx, userID, "", content, auditStageCommentBody)
}

// invokeResult 记录模型调用结果以及免费额度的使用情况。
type invokeResult struct {
	Response     modeldomain.ChatCompletionResponse
	FreeTierUsed bool
	FreeTierInfo *freeTierUsage
}

// invokeModelWithFallback 优先使用用户自定义凭据调用模型，若缺失则回退到免费额度。
//  1. 先调用原来的 model.InvokeChatCompletion（也就是 ModelService.InvokeChatCompletion），这和过去的逻辑完全一样。
//  2. 如果返回 modelsvc.ErrCredentialNotFound 或 ErrCredentialDisabled，并且我们启用了 free tier，就改走 freeTier.invoke(...)。
//  3. 其它错误保持原状向上抛，让 Handler 决定应该提示网络错误还是内容审核失败。
func (s *Service) invokeModelWithFallback(ctx context.Context, userID uint, modelKey string, req modeldomain.ChatCompletionRequest) (invokeResult, error) {
	if s.model == nil {
		return invokeResult{}, fmt.Errorf("%w: 模型服务未初始化", ErrModelInvocationFailed)
	}
	resp, err := s.model.InvokeChatCompletion(ctx, userID, modelKey, req)
	if err == nil {
		return invokeResult{Response: resp}, nil
	}

	if s.freeTier == nil || !s.freeTier.matches(modelKey) {
		return invokeResult{}, fmt.Errorf("%w: %w", ErrModelInvocationFailed, err)
	}
	if !errors.Is(err, modelsvc.ErrCredentialNotFound) && !errors.Is(err, modelsvc.ErrCredentialDisabled) {
		return invokeResult{}, fmt.Errorf("%w: %w", ErrModelInvocationFailed, err)
	}

	resp, usage, fallbackErr := s.freeTier.invoke(ctx, userID, req)
	if fallbackErr != nil {
		if quotaErr := (*FreeTierQuotaExceededError)(nil); errors.As(fallbackErr, &quotaErr) {
			return invokeResult{}, quotaErr
		}
		return invokeResult{}, fmt.Errorf("%w: %w", ErrModelInvocationFailed, fallbackErr)
	}

	return invokeResult{
		Response:     resp,
		FreeTierUsed: true,
		FreeTierInfo: &usage,
	}, nil
}

// SaveInput 描述保存草稿或发布 Prompt 的参数。
type SaveInput struct {
	UserID                   uint
	PromptID                 uint
	Topic                    string
	Body                     string
	Instructions             string
	Model                    string
	Status                   string
	PositiveKeywords         []KeywordItem
	NegativeKeywords         []KeywordItem
	Tags                     []string
	Publish                  bool
	WorkspaceToken           string
	EnforcePublishValidation bool
}

// SaveOutput 返回保存后的 Prompt 元数据。
type SaveOutput struct {
	PromptID uint   `json:"prompt_id"`
	Status   string `json:"status"`
	Version  int    `json:"version"`
	TaskID   string `json:"task_id,omitempty"`
	Token    string `json:"workspace_token,omitempty"`
}

// Interpret 调用大模型解析自然语言描述，并将结果写入关键词表以便复用。
func (s *Service) Interpret(ctx context.Context, input InterpretInput) (InterpretOutput, error) {
	description := strings.TrimSpace(input.Description)
	if description == "" {
		return InterpretOutput{}, errors.New("description is empty")
	}
	modelKey := strings.TrimSpace(input.ModelKey)
	if modelKey == "" {
		if alias := s.freeTier.defaultAlias(); alias != "" {
			modelKey = alias
		} else {
			return InterpretOutput{}, errors.New("model key is empty")
		}
	}
	if err := s.auditContent(ctx, input.UserID, modelKey, description, auditStageInterpretInput); err != nil {
		return InterpretOutput{}, err
	}
	req := buildInterpretationRequest(description, input.Language)
	req.Model = modelKey
	// 防止模型超时
	modelCtx, cancel := s.modelInvocationContext(ctx)
	defer cancel()
	invokeRes, err := s.invokeModelWithFallback(modelCtx, input.UserID, modelKey, req)
	if err != nil {
		return InterpretOutput{}, err
	}

	payload, err := parseInterpretationPayload(invokeRes.Response)
	if err != nil {
		return InterpretOutput{}, err
	}

	cleanedTags := []string{}
	if len(payload.Tags) > 0 {
		if tags, err := s.normalizeTags(payload.Tags); err != nil {
			cleanedTags = s.truncateTags(payload.Tags)
		} else {
			cleanedTags = tags
		}
	}
	output := InterpretOutput{
		Topic:        payload.Topic,
		Confidence:   payload.Confidence,
		Instructions: payload.Instructions,
		Tags:         cleanedTags,
	}
	if output.Topic == "" {
		return InterpretOutput{}, errors.New("model did not return topic")
	}
	normalized := newKeywordSet()
	for _, entry := range payload.Positive {
		item := KeywordItem{
			Word:     entry.Word,
			Source:   promptdomain.KeywordSourceModel,
			Polarity: promptdomain.KeywordPolarityPositive,
			Weight:   clampWeight(entry.Weight),
		}
		item.Word = s.clampKeywordWord(item.Word)
		if normalized.add(item) {
			output.PositiveKeywords = append(output.PositiveKeywords, item)
		}
	}
	for _, entry := range payload.Negative {
		item := KeywordItem{
			Word:     entry.Word,
			Source:   promptdomain.KeywordSourceModel,
			Polarity: promptdomain.KeywordPolarityNegative,
			Weight:   clampWeight(entry.Weight),
		}
		item.Word = s.clampKeywordWord(item.Word)
		if normalized.add(item) {
			output.NegativeKeywords = append(output.NegativeKeywords, item)
		}
	}

	if s.workspace != nil {
		storeCtx, cancel := s.workspaceContext(ctx)
		defer cancel()
		workspaceSnapshot := promptdomain.WorkspaceSnapshot{
			Token:     "",
			UserID:    input.UserID,
			Topic:     payload.Topic,
			Language:  input.Language,
			ModelKey:  modelKey,
			DraftBody: "",
			Positive:  toWorkspaceKeywords(output.PositiveKeywords, s.keywordLimit),
			Negative:  toWorkspaceKeywords(output.NegativeKeywords, s.keywordLimit),
			Version:   1,
		}
		attributes := map[string]string{}
		if strings.TrimSpace(output.Instructions) != "" {
			attributes[workspaceAttrInstructions] = output.Instructions
		}
		if len(output.Tags) > 0 {
			attributes[workspaceAttrTags] = encodeTagsAttribute(output.Tags)
		}
		if len(attributes) > 0 {
			workspaceSnapshot.Attributes = attributes
		}
		if token, err := s.workspace.CreateOrReplace(storeCtx, input.UserID, workspaceSnapshot); err != nil {
			s.logger.Warnw("store workspace snapshot failed", "user_id", input.UserID, "topic", payload.Topic, "error", err)
		} else {
			output.WorkspaceToken = token
		}
	} else {
		// 无 Redis 时保持旧行为，直接写入 MySQL 字典。
		s.persistKeywords(ctx, input.UserID, payload.Topic, append(output.PositiveKeywords, output.NegativeKeywords...))
	}

	if len(output.PositiveKeywords) == 0 {
		return InterpretOutput{}, errors.New("model did not return positive keywords")
	}
	if len(output.PositiveKeywords) > s.keywordLimit {
		output.PositiveKeywords = output.PositiveKeywords[:s.keywordLimit]
	}
	if len(output.NegativeKeywords) > s.keywordLimit {
		output.NegativeKeywords = output.NegativeKeywords[:s.keywordLimit]
	}

	return output, nil
}

// AugmentKeywords 调用模型补充关键词，并返回真正新增的词条，同时维持去重与上限控制。
func (s *Service) AugmentKeywords(ctx context.Context, input AugmentInput) (AugmentOutput, error) {
	if strings.TrimSpace(input.Topic) == "" {
		return AugmentOutput{}, errors.New("topic is empty")
	}
	modelKey := strings.TrimSpace(input.ModelKey)
	if modelKey == "" {
		if alias := s.freeTier.defaultAlias(); alias != "" {
			modelKey = alias
		} else {
			return AugmentOutput{}, errors.New("model key is empty")
		}
	}
	positiveCapacity := s.keywordLimit - len(input.ExistingPositive)
	if positiveCapacity < 0 {
		positiveCapacity = 0
	}
	negativeCapacity := s.keywordLimit - len(input.ExistingNegative)
	if negativeCapacity < 0 {
		negativeCapacity = 0
	}
	if positiveCapacity == 0 && negativeCapacity == 0 {
		return AugmentOutput{}, nil
	}

	req := buildAugmentRequest(input)
	req.Model = modelKey
	modelCtx, cancel := s.modelInvocationContext(ctx)
	defer cancel()
	invokeRes, err := s.invokeModelWithFallback(modelCtx, input.UserID, modelKey, req)
	if err != nil {
		return AugmentOutput{}, err
	}
	payload, err := parseAugmentPayload(invokeRes.Response)
	if err != nil {
		return AugmentOutput{}, err
	}

	existing := newKeywordSet()
	for _, item := range append(input.ExistingPositive, input.ExistingNegative...) {
		existing.add(item)
	}

	var (
		output           AugmentOutput
		workspaceNew     []promptdomain.WorkspaceKeyword
		workspaceEnabled = s.workspace != nil && strings.TrimSpace(input.WorkspaceToken) != ""
	)
	for idx, entry := range payload.Positive {
		if positiveCapacity <= 0 {
			break
		}
		item := KeywordItem{
			Word:     entry.Word,
			Source:   promptdomain.KeywordSourceModel,
			Polarity: promptdomain.KeywordPolarityPositive,
			Weight:   clampWeight(entry.Weight),
		}
		item.Word = s.clampKeywordWord(item.Word)
		if existing.add(item) {
			output.Positive = append(output.Positive, item)
			positiveCapacity--
			if workspaceEnabled {
				workspaceNew = append(workspaceNew, promptdomain.WorkspaceKeyword{
					Word:     strings.TrimSpace(item.Word),
					Source:   sourceFallback(item.Source),
					Polarity: promptdomain.KeywordPolarityPositive,
					Weight:   clampWeight(entry.Weight),
					Score:    float64(time.Now().UnixNano()) + float64(idx),
				})
			} else {
				if _, err := s.keywords.Upsert(ctx, toKeywordEntity(input.UserID, input.Topic, item)); err != nil {
					s.logger.Warnw("upsert keyword failed", "topic", input.Topic, "word", item.Word, "error", err)
				}
			}
		}
	}
	for idx, entry := range payload.Negative {
		if negativeCapacity <= 0 {
			break
		}
		item := KeywordItem{
			Word:     entry.Word,
			Source:   promptdomain.KeywordSourceModel,
			Polarity: promptdomain.KeywordPolarityNegative,
			Weight:   clampWeight(entry.Weight),
		}
		item.Word = s.clampKeywordWord(item.Word)
		if existing.add(item) {
			output.Negative = append(output.Negative, item)
			negativeCapacity--
			if workspaceEnabled {
				workspaceNew = append(workspaceNew, promptdomain.WorkspaceKeyword{
					Word:     strings.TrimSpace(item.Word),
					Source:   sourceFallback(item.Source),
					Polarity: promptdomain.KeywordPolarityNegative,
					Weight:   clampWeight(entry.Weight),
					Score:    float64(time.Now().UnixNano()) + float64(idx),
				})
			} else {
				if _, err := s.keywords.Upsert(ctx, toKeywordEntity(input.UserID, input.Topic, item)); err != nil {
					s.logger.Warnw("upsert keyword failed", "topic", input.Topic, "word", item.Word, "error", err)
				}
			}
		}
	}
	if workspaceEnabled && len(workspaceNew) > 0 {
		storeCtx, cancel := s.workspaceContext(ctx)
		defer cancel()
		if err := s.workspace.MergeKeywords(storeCtx, input.UserID, input.WorkspaceToken, workspaceNew); err != nil {
			s.logger.Warnw("merge workspace keywords failed", "user_id", input.UserID, "token", input.WorkspaceToken, "error", err)
		} else if err := s.workspace.Touch(storeCtx, input.UserID, input.WorkspaceToken); err != nil {
			s.logger.Warnw("touch workspace failed", "user_id", input.UserID, "token", input.WorkspaceToken, "error", err)
		}
	}
	return output, nil
}

// AddManualKeyword 将用户手动输入的关键词写入数据库，并返回最终条目。
func (s *Service) AddManualKeyword(ctx context.Context, input ManualKeywordInput) (KeywordItem, error) {
	word := s.clampKeywordWord(input.Word)
	if word == "" {
		return KeywordItem{}, errors.New("keyword is empty")
	}
	if input.UserID == 0 {
		return KeywordItem{}, errors.New("user id is required")
	}
	if strings.TrimSpace(input.Topic) == "" {
		return KeywordItem{}, errors.New("topic is required")
	}
	polarity := normalizePolarity(input.Polarity)
	source := input.Source
	if source == "" {
		source = promptdomain.KeywordSourceManual
	}
	weight := clampWeight(func(val int) int {
		if val <= 0 {
			return defaultKeywordWeight
		}
		return val
	}(input.Weight))
	if s.workspace != nil && strings.TrimSpace(input.WorkspaceToken) != "" {
		token := strings.TrimSpace(input.WorkspaceToken)
		storeCtx, cancel := s.workspaceContext(ctx)
		defer cancel()
		positiveCount := 0
		negativeCount := 0
		snapshot, snapErr := s.workspace.Snapshot(storeCtx, input.UserID, token)
		if snapErr == nil {
			positiveCount = workspaceKeywordCount(snapshot.Positive)
			negativeCount = workspaceKeywordCount(snapshot.Negative)
			if workspaceHasKeyword(snapshot.Positive, promptdomain.KeywordPolarityPositive, word) && polarity == promptdomain.KeywordPolarityPositive {
				return KeywordItem{}, ErrDuplicateKeyword
			}
			if workspaceHasKeyword(snapshot.Negative, promptdomain.KeywordPolarityNegative, word) && polarity == promptdomain.KeywordPolarityNegative {
				return KeywordItem{}, ErrDuplicateKeyword
			}
		} else if !errors.Is(snapErr, redis.Nil) {
			s.logger.Warnw("load workspace snapshot for manual keyword failed", "user_id", input.UserID, "token", token, "error", snapErr)
		}
		if polarity == promptdomain.KeywordPolarityPositive && positiveCount >= s.keywordLimit {
			return KeywordItem{}, ErrPositiveKeywordLimit
		}
		if polarity == promptdomain.KeywordPolarityNegative && negativeCount >= s.keywordLimit {
			return KeywordItem{}, ErrNegativeKeywordLimit
		}
		workspaceKeyword := promptdomain.WorkspaceKeyword{
			Word:     word,
			Source:   source,
			Polarity: polarity,
			Weight:   weight,
			Score:    float64(time.Now().UnixNano()),
		}
		if err := s.workspace.MergeKeywords(storeCtx, input.UserID, token, []promptdomain.WorkspaceKeyword{workspaceKeyword}); err != nil {
			s.logger.Warnw("merge manual keyword to workspace failed", "user_id", input.UserID, "token", token, "word", word, "error", err)
		} else if err := s.workspace.Touch(storeCtx, input.UserID, token); err != nil {
			s.logger.Warnw("touch workspace failed", "user_id", input.UserID, "token", token, "error", err)
		}
		return KeywordItem{
			KeywordID: 0,
			Word:      word,
			Source:    source,
			Polarity:  polarity,
			Weight:    weight,
		}, nil
	}

	entity := toKeywordEntity(input.UserID, input.Topic, KeywordItem{
		Word:     word,
		Source:   source,
		Polarity: polarity,
		Weight:   weight,
	})
	if lang := strings.TrimSpace(input.Language); lang != "" {
		entity.Language = lang
	}
	stored, err := s.keywords.Upsert(ctx, entity)
	if err != nil {
		return KeywordItem{}, err
	}

	item := KeywordItem{
		KeywordID: stored.ID,
		Word:      stored.Word,
		Source:    stored.Source,
		Polarity:  stored.Polarity,
		Weight:    clampWeight(stored.Weight),
	}
	if input.PromptID != 0 {
		if err := s.keywords.AttachToPrompt(ctx, input.PromptID, stored.ID, relationByPolarity(item.Polarity)); err != nil {
			s.logger.Warnw("attach manual keyword failed", "promptID", input.PromptID, "keywordID", stored.ID, "error", err)
		}
	}
	return item, nil
}

// RemoveWorkspaceKeyword 从临时工作区中移除单个关键词，保持 Redis 与前端状态同步。
func (s *Service) RemoveWorkspaceKeyword(ctx context.Context, input RemoveKeywordInput) error {
	if s.workspace == nil {
		return nil
	}
	token := strings.TrimSpace(input.WorkspaceToken)
	if token == "" {
		return nil
	}
	word := strings.TrimSpace(input.Word)
	if word == "" {
		return errors.New("keyword is empty")
	}
	storeCtx, cancel := s.workspaceContext(ctx)
	defer cancel()
	if err := s.workspace.RemoveKeyword(storeCtx, input.UserID, token, normalizePolarity(input.Polarity), word); err != nil {
		return err
	}
	return nil
}

// SyncWorkspaceKeywords 是前端拖拽关键词、调整权重后同步 Redis 工作区快照的接口。保持 Redis 状态与 UI 一致。流程如下：
// 1. 前端把最新的正/负向关键词列表（包含顺序与权重）连同 workspace_token 发给后端。
// 2. Service 先用 workspace.Snapshot 取回当前 Redis 快照：里面包括草稿正文、现有关键词等。
// 3. 用 workspaceKeywordsFromOrdered 把前端传回的数组转换成 Redis 存储结构（我们会重新分配 score、清洗来源/权重）。
// 4. 更新 snapshot 的 Positive/Negative 列表、更新时间和版本号。
// 5. 调用 workspace.CreateOrReplace，这一步内部使用 MULTI/EXEC 管道一次性覆盖 Hash 和 ZSet，保证排序与权重同步更新。
// 6. 如果 token 失效或 Redis 有问题就返回错误，让前端重新拉最新快照。
func (s *Service) SyncWorkspaceKeywords(ctx context.Context, input SyncWorkspaceInput) error {
	if s.workspace == nil {
		return nil
	}
	token := strings.TrimSpace(input.WorkspaceToken)
	if token == "" {
		return errors.New("workspace token is empty")
	}
	if err := enforceKeywordLimit(s.keywordLimit, input.Positive, input.Negative); err != nil {
		return err
	}
	storeCtx, cancel := s.workspaceContext(ctx)
	defer cancel()
	snapshot, err := s.workspace.Snapshot(storeCtx, input.UserID, token)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return fmt.Errorf("workspace not found")
		}
		return err
	}
	snapshot.Positive = s.workspaceKeywordsFromOrdered(input.Positive)
	snapshot.Negative = s.workspaceKeywordsFromOrdered(input.Negative)
	snapshot.UpdatedAt = time.Now()
	snapshot.Version++
	snapshot.Token = token
	if _, err := s.workspace.CreateOrReplace(storeCtx, input.UserID, snapshot); err != nil {
		return err
	}
	return nil
}

// GeneratePrompt 调用模型生成 Prompt，并返回正文与耗时。
func (s *Service) GeneratePrompt(ctx context.Context, input GenerateInput) (GenerateOutput, error) {
	if strings.TrimSpace(input.Topic) == "" {
		return GenerateOutput{}, errors.New("topic is empty")
	}
	modelKey := strings.TrimSpace(input.ModelKey)
	if modelKey == "" {
		if alias := s.freeTier.defaultAlias(); alias != "" {
			modelKey = alias
		} else {
			return GenerateOutput{}, errors.New("model key is empty")
		}
	}
	if len(input.PositiveKeywords) == 0 {
		return GenerateOutput{}, errors.New("positive keywords required")
	}
	if err := enforceKeywordLimit(s.keywordLimit, input.PositiveKeywords, input.NegativeKeywords); err != nil {
		return GenerateOutput{}, err
	}
	req := buildGenerateRequest(input)
	req.Model = modelKey
	start := time.Now()
	modelCtx, cancel := s.modelInvocationContext(ctx)
	defer cancel()
	invokeRes, err := s.invokeModelWithFallback(modelCtx, input.UserID, modelKey, req)
	if err != nil {
		return GenerateOutput{}, err
	}
	duration := time.Since(start)
	promptText := extractPromptText(invokeRes.Response)
	if promptText == "" {
		return GenerateOutput{}, errors.New("model returned empty prompt")
	}
	if err := s.auditContent(ctx, input.UserID, modelKey, promptText, auditStageGenerateOutput); err != nil {
		return GenerateOutput{}, err
	}
	if s.workspace != nil && strings.TrimSpace(input.WorkspaceToken) != "" {
		// 写回最新草稿并刷新 TTL，防止用户在生成后继续调整时工作区被 Redis 过期策略清理。
		storeCtx, cancelStore := s.workspaceContext(ctx)
		defer cancelStore()
		if err := s.workspace.UpdateDraftBody(storeCtx, input.UserID, strings.TrimSpace(input.WorkspaceToken), promptText); err != nil {
			s.logger.Warnw("update workspace draft failed", "user_id", input.UserID, "token", input.WorkspaceToken, "error", err)
		} else if err := s.workspace.Touch(storeCtx, input.UserID, strings.TrimSpace(input.WorkspaceToken)); err != nil {
			s.logger.Warnw("touch workspace failed", "user_id", input.UserID, "token", input.WorkspaceToken, "error", err)
		}
	}
	return GenerateOutput{
		Model:        invokeRes.Response.Model,
		Prompt:       promptText,
		Duration:     duration,
		Usage:        invokeRes.Response.Usage,
		PositiveUsed: input.PositiveKeywords,
		NegativeUsed: input.NegativeKeywords,
	}, nil
}

// Save 保存或发布 Prompt：
//  1. 若绑定了 Redis 工作区则合并快照内容（主题/正文/关键词/标签等）
//  2. 校验关键词 & 标签数量上限
//  3. 创建/更新 Prompt 主记录与关键词表
//  4. 回写工作区元数据（prompt_id/status/tags）保持前端同步
func (s *Service) Save(ctx context.Context, input SaveInput) (SaveOutput, error) {
	if input.UserID == 0 {
		return SaveOutput{}, errors.New("user id required")
	}
	status := normalizeStatus(input.Status, input.Publish)
	workspaceToken := strings.TrimSpace(input.WorkspaceToken)
	workspaceEnabled := s.workspace != nil && workspaceToken != ""

	var snapshot promptdomain.WorkspaceSnapshot
	if workspaceEnabled {
		storeCtx, cancel := s.workspaceContext(ctx)
		snap, err := s.workspace.Snapshot(storeCtx, input.UserID, workspaceToken)
		cancel()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				s.logger.Warnw("workspace snapshot expired", "user_id", input.UserID, "token", workspaceToken)
				workspaceEnabled = false
			} else {
				s.logger.Warnw("load workspace snapshot failed", "user_id", input.UserID, "token", workspaceToken, "error", err)
				workspaceEnabled = false
			}
		} else {
			snapshot = snap
			if strings.TrimSpace(input.Topic) == "" {
				input.Topic = snapshot.Topic
			}
			if strings.TrimSpace(input.Model) == "" && strings.TrimSpace(snapshot.ModelKey) != "" {
				input.Model = snapshot.ModelKey
			}
			if strings.TrimSpace(input.Body) == "" {
				input.Body = snapshot.DraftBody
			}
			if len(input.PositiveKeywords) == 0 {
				input.PositiveKeywords = s.keywordItemsFromWorkspace(snapshot.Positive)
			}
			if len(input.NegativeKeywords) == 0 {
				input.NegativeKeywords = s.keywordItemsFromWorkspace(snapshot.Negative)
			}
			if len(input.Tags) == 0 {
				if tags := extractTagsFromAttributes(snapshot.Attributes); len(tags) > 0 {
					input.Tags = s.truncateTags(tags)
				}
			}
			if strings.TrimSpace(input.Instructions) == "" {
				if instr := extractInstructionsFromAttributes(snapshot.Attributes); instr != "" {
					input.Instructions = instr
				}
			}
			if snapshot.PromptID != 0 && input.PromptID == 0 {
				input.PromptID = snapshot.PromptID
			}
			if strings.TrimSpace(input.Status) == "" && strings.TrimSpace(snapshot.Status) != "" {
				status = normalizeStatus(snapshot.Status, input.Publish)
			}
		}
	}

	topicLookup := normalizeMixedLanguageSpacing(strings.TrimSpace(input.Topic))
	if input.PromptID == 0 && topicLookup != "" {
		if existing, err := s.prompts.FindByUserAndTopic(ctx, input.UserID, topicLookup); err == nil {
			input.PromptID = existing.ID
		}
	}

	if err := enforceKeywordLimit(s.keywordLimit, input.PositiveKeywords, input.NegativeKeywords); err != nil {
		return SaveOutput{}, err
	}

	cleanedTags, err := s.normalizeTags(input.Tags)
	if err != nil {
		return SaveOutput{}, err
	}
	input.Tags = cleanedTags
	input.Instructions = strings.TrimSpace(input.Instructions)

	action := promptdomain.TaskActionCreate
	if input.PromptID != 0 {
		action = promptdomain.TaskActionUpdate
	}

	result, err := s.persistPrompt(ctx, input, status, action)
	if err != nil {
		return SaveOutput{}, err
	}

	if workspaceEnabled {
		metaCtx, cancel := s.workspaceContext(ctx)
		attrs := map[string]string{
			workspaceAttrTags:         encodeTagsAttribute(input.Tags),
			workspaceAttrInstructions: input.Instructions,
		}
		if err := s.workspace.SetAttributes(metaCtx, input.UserID, workspaceToken, attrs); err != nil {
			s.logger.Warnw("set workspace tags failed", "user_id", input.UserID, "token", workspaceToken, "error", err)
		}
		if err := s.workspace.SetPromptMeta(metaCtx, input.UserID, workspaceToken, result.PromptID, status); err != nil {
			s.logger.Warnw("set workspace meta failed", "user_id", input.UserID, "token", workspaceToken, "error", err)
		}
		if err := s.workspace.Touch(metaCtx, input.UserID, workspaceToken); err != nil {
			s.logger.Warnw("touch workspace failed", "user_id", input.UserID, "token", workspaceToken, "error", err)
		}
		cancel()
		result.Token = workspaceToken
	}

	return result, nil
}

// helper: 构造关键词实体。
func toKeywordEntity(userID uint, topic string, item KeywordItem) *promptdomain.Keyword {
	topicValue := normalizeMixedLanguageSpacing(strings.TrimSpace(topic))
	return &promptdomain.Keyword{
		UserID:   userID,
		Topic:    topicValue,
		Word:     strings.TrimSpace(item.Word),
		Polarity: normalizePolarity(item.Polarity),
		Source:   sourceFallback(item.Source),
		Language: "zh",
		Weight:   clampWeight(item.Weight),
	}
}

// toWorkspaceKeywords 将关键词列表裁剪并转换为 Redis 工作区使用的结构体，确保不会超过上限。
func toWorkspaceKeywords(items []KeywordItem, limit int) []promptdomain.WorkspaceKeyword {
	if limit <= 0 {
		limit = DefaultKeywordLimit
	}
	if len(items) < limit {
		limit = len(items)
	}
	result := make([]promptdomain.WorkspaceKeyword, 0, limit)
	for idx := 0; idx < limit; idx++ {
		item := items[idx]
		result = append(result, promptdomain.WorkspaceKeyword{
			Word:     strings.TrimSpace(item.Word),
			Source:   sourceFallback(item.Source),
			Polarity: normalizePolarity(item.Polarity),
			Weight:   clampWeight(item.Weight),
			Score:    float64(idx + 1),
		})
	}
	return result
}

// workspaceKeywordsFromOrdered 根据 UI 提交的顺序重建工作区关键词列表，用于拖拽/调权之后的同步。
//   - 输入：前端提交的 KeywordItem 数组（包含 word/source/weight/polarity），顺序就是 UI 里显示的顺序。
//   - 处理：
//   - 先根据 PROMPT_KEYWORD_LIMIT 截断，防止写入超过上限的条目。
//   - 对每个词做清洗（clampKeywordWord 限制长度、normalizePolarity 统一正负、sourceFallback 兼容来源）。
//   - 把数组下标 +1 作为 Score，用来写入 ZSet，确保排序和 UI 一致。
//   - 输出：[]WorkspaceKeyword，里头包含 Word、Source、Polarity、Weight、Score，这个结构正好用于写入 Redis 的 Hash 和 ZSet。
func (s *Service) workspaceKeywordsFromOrdered(items []KeywordItem) []promptdomain.WorkspaceKeyword {
	limit := s.keywordLimit
	if limit <= 0 {
		limit = DefaultKeywordLimit
	}
	capacity := limit
	if len(items) < capacity {
		capacity = len(items)
	}
	result := make([]promptdomain.WorkspaceKeyword, 0, capacity)
	for idx, item := range items {
		if idx >= limit {
			break
		}
		clamped := s.clampKeywordWord(item.Word)
		result = append(result, promptdomain.WorkspaceKeyword{
			Word:     clamped,
			Source:   sourceFallback(item.Source),
			Polarity: normalizePolarity(item.Polarity),
			Weight:   clampWeight(item.Weight),
			Score:    float64(idx + 1),
		})
	}
	return result
}

// keywordItemsFromWorkspace 将工作区缓存的关键词还原为业务层结构体。
func (s *Service) keywordItemsFromWorkspace(items []promptdomain.WorkspaceKeyword) []KeywordItem {
	limit := s.keywordLimit
	if limit <= 0 {
		limit = DefaultKeywordLimit
	}
	result := make([]KeywordItem, 0, len(items))
	for _, item := range items {
		word := s.clampKeywordWord(item.Word)
		result = append(result, KeywordItem{
			Word:     word,
			Source:   sourceFallback(item.Source),
			Polarity: normalizePolarity(item.Polarity),
			Weight:   clampWeight(item.Weight),
		})
		if len(result) >= limit {
			break
		}
	}
	return result
}

// firstNonEmpty 返回第一个非空字符串，常用于从多个候选值里挑选有效字段。
func firstNonEmpty(values ...string) string {
	for _, val := range values {
		if strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

// normalizePolarity 将输入规范到 positive/negative，默认 positive。
func normalizePolarity(polarity string) string {
	switch strings.ToLower(strings.TrimSpace(polarity)) {
	case promptdomain.KeywordPolarityNegative:
		return promptdomain.KeywordPolarityNegative
	default:
		return promptdomain.KeywordPolarityPositive
	}
}

// relationByPolarity 根据极性决定正/负向关系，落库时用于区分关键词类型。
func relationByPolarity(polarity string) string {
	if normalizePolarity(polarity) == promptdomain.KeywordPolarityNegative {
		return promptdomain.KeywordPolarityNegative
	}
	return promptdomain.KeywordPolarityPositive
}

// sourceFallback 在缺失来源时回落到手动标签，保持字段完整性。
// sourceFallback 在关键词来源缺失时使用“manual”作为默认值，保证字段完整。
func sourceFallback(source string) string {
	if source == "" {
		return promptdomain.KeywordSourceManual
	}
	return source
}

// enforceKeywordLimit 统一校验关键词数量是否超过上限，避免后续流程写入异常数据。
func enforceKeywordLimit(limit int, positive, negative []KeywordItem) error {
	if len(positive) > limit {
		return ErrPositiveKeywordLimit
	}
	if len(negative) > limit {
		return ErrNegativeKeywordLimit
	}
	return nil
}

// workspaceKeywordCount 统计工作区内的唯一关键词数量，忽略大小写与前后空格差异。
func workspaceKeywordCount(items []promptdomain.WorkspaceKeyword) int {
	if len(items) == 0 {
		return 0
	}
	seen := make(map[string]struct{})
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item.Word))
		if key == "" {
			continue
		}
		seen[key] = struct{}{}
	}
	return len(seen)
}

// workspaceHasKeyword 判断指定极性下是否已存在目标关键词，用于拦截重复添加。
func workspaceHasKeyword(items []promptdomain.WorkspaceKeyword, polarity, word string) bool {
	target := normalizePolarity(polarity)
	needle := strings.ToLower(strings.TrimSpace(word))
	if needle == "" {
		return false
	}
	for _, item := range items {
		if normalizePolarity(item.Polarity) != target {
			continue
		}
		if strings.ToLower(strings.TrimSpace(item.Word)) == needle {
			return true
		}
	}
	return false
}

// persistKeywords 将临时关键词落入数据库，便于后续复用。
func (s *Service) persistKeywords(ctx context.Context, userID uint, topic string, items []KeywordItem) {
	sanitizedTopic := normalizeMixedLanguageSpacing(strings.TrimSpace(topic))
	for _, item := range items {
		if _, err := s.keywords.Upsert(ctx, toKeywordEntity(userID, sanitizedTopic, item)); err != nil {
			s.logger.Warnw("upsert keyword failed", "topic", sanitizedTopic, "word", item.Word, "error", err)
		}
	}
}

// processPersistenceTask 消费异步队列任务，将 Redis 工作区内容（含标签）持久化回数据库。
// 把快照和任务里的字段合成一个完整的 SaveInput，补齐标签去重、关键词列表之类的细节，然后再调用 persistPrompt
func (s *Service) processPersistenceTask(ctx context.Context, task promptdomain.PersistenceTask) error {
	if s.workspace == nil {
		return errors.New("workspace store not configured")
	}
	storeCtx, cancel := s.workspaceContext(ctx)
	snapshot, err := s.workspace.Snapshot(storeCtx, task.UserID, task.WorkspaceToken)
	cancel()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return fmt.Errorf("workspace not found for task %s", task.TaskID)
		}
		return fmt.Errorf("load workspace snapshot: %w", err)
	}
	input := SaveInput{
		UserID:                   task.UserID,
		PromptID:                 task.PromptID,
		Topic:                    firstNonEmpty(task.Topic, snapshot.Topic),
		Body:                     firstNonEmpty(task.Body, snapshot.DraftBody),
		Instructions:             firstNonEmpty(task.Instructions, extractInstructionsFromAttributes(snapshot.Attributes)),
		Model:                    firstNonEmpty(task.Model, snapshot.ModelKey),
		Status:                   task.Status,
		PositiveKeywords:         s.keywordItemsFromWorkspace(snapshot.Positive),
		NegativeKeywords:         s.keywordItemsFromWorkspace(snapshot.Negative),
		Tags:                     task.Tags,
		Publish:                  task.Publish,
		EnforcePublishValidation: true,
	}
	if len(input.Tags) == 0 {
		if tags := extractTagsFromAttributes(snapshot.Attributes); len(tags) > 0 {
			input.Tags = s.truncateTags(tags)
		}
	}
	if input.PromptID == 0 && snapshot.PromptID != 0 {
		input.PromptID = snapshot.PromptID
	}
	action := strings.TrimSpace(task.Action)
	if action == "" {
		if input.PromptID == 0 {
			action = promptdomain.TaskActionCreate
		} else {
			action = promptdomain.TaskActionUpdate
		}
	}
	status := normalizeStatus(task.Status, task.Publish)
	if err := enforceKeywordLimit(s.keywordLimit, input.PositiveKeywords, input.NegativeKeywords); err != nil {
		return fmt.Errorf("keyword limit: %w", err)
	}
	cleanedTags, err := s.normalizeTags(input.Tags)
	if err != nil {
		return fmt.Errorf("tag limit: %w", err)
	}
	input.Tags = cleanedTags
	result, err := s.persistPrompt(ctx, input, status, action)
	if err != nil {
		return fmt.Errorf("persist prompt: %w", err)
	}
	metaCtx, cancelMeta := s.workspaceContext(ctx)
	attrs := map[string]string{
		workspaceAttrTags:         encodeTagsAttribute(input.Tags),
		workspaceAttrInstructions: input.Instructions,
	}
	if err := s.workspace.SetAttributes(metaCtx, task.UserID, task.WorkspaceToken, attrs); err != nil {
		s.logger.Warnw("set workspace tags failed", "task_id", task.TaskID, "token", task.WorkspaceToken, "error", err)
	}
	if err := s.workspace.SetPromptMeta(metaCtx, task.UserID, task.WorkspaceToken, result.PromptID, status); err != nil {
		s.logger.Warnw("set workspace meta failed", "task_id", task.TaskID, "token", task.WorkspaceToken, "error", err)
	}
	cancelMeta()
	s.logger.Infow("prompt persisted", "task_id", task.TaskID, "prompt_id", result.PromptID, "user_id", task.UserID, "publish", task.Publish)
	return nil
}

// persistPrompt 根据动作类型创建或更新 Prompt 主记录，同时保持关键词与版本一致。
// persistPrompt 是 Prompt 服务里的核心持久化方法。它负责把一次“保存/发布”请求整理成落库动作：先清洗 Topic、正文、补充要求、模型、标签，再在需要发布时触发字段校验（比如主题、正文、补充要求、模型、正/负关键词、标签是否齐全），最后根据动作类型调用 createPromptRecord 或 updatePromptRecord 去写入主表、关键词表和版本信息。
func (s *Service) persistPrompt(ctx context.Context, input SaveInput, status, action string) (SaveOutput, error) {
	input.Topic = strings.TrimSpace(input.Topic)
	input.Body = strings.TrimSpace(input.Body)
	input.Instructions = strings.TrimSpace(input.Instructions)
	input.Model = strings.TrimSpace(input.Model)
	if err := enforceKeywordLimit(s.keywordLimit, input.PositiveKeywords, input.NegativeKeywords); err != nil {
		return SaveOutput{}, err
	}
	cleanedTags, err := s.normalizeTags(input.Tags)
	if err != nil {
		return SaveOutput{}, err
	}
	input.Tags = cleanedTags
	// 只有当这次保存的最终状态是 published，并且调用方显式要求执行发布校验（EnforcePublishValidation == true）时，才会去跑
	// validatePublishInput。validatePublishInput 会检查发布必须具备的字段，例如主题、正文、补充要求、模型、正/负向关键词、标签等。一旦缺少，就返回错误，
	// 阻止这次发布
	if status == promptdomain.PromptStatusPublished && input.EnforcePublishValidation {
		if err := s.validatePublishInput(input); err != nil {
			return SaveOutput{}, err
		}
	}
	action = strings.TrimSpace(action)
	if action == "" {
		if input.PromptID == 0 {
			action = promptdomain.TaskActionCreate
		} else {
			action = promptdomain.TaskActionUpdate
		}
	}
	switch action {
	case promptdomain.TaskActionUpdate:
		return s.updatePromptRecord(ctx, input, status)
	default:
		return s.createPromptRecord(ctx, input, status)
	}
}

// createPromptRecord 写入新的 Prompt，并在必要时记录首个版本。
func (s *Service) createPromptRecord(ctx context.Context, input SaveInput, status string) (SaveOutput, error) {
	encodedPos, err := s.marshalKeywordItems(input.PositiveKeywords)
	if err != nil {
		return SaveOutput{}, err
	}
	encodedNeg, err := s.marshalKeywordItems(input.NegativeKeywords)
	if err != nil {
		return SaveOutput{}, err
	}
	encodedTags, err := json.Marshal(input.Tags)
	if err != nil {
		return SaveOutput{}, fmt.Errorf("encode tags: %w", err)
	}
	topic := normalizeMixedLanguageSpacing(input.Topic)
	entity := &promptdomain.Prompt{
		UserID:           input.UserID,
		Topic:            topic,
		Body:             input.Body,
		Instructions:     input.Instructions,
		PositiveKeywords: string(encodedPos),
		NegativeKeywords: string(encodedNeg),
		Model:            strings.TrimSpace(input.Model),
		Status:           status,
		Tags:             string(encodedTags),
		LatestVersionNo:  0,
	}
	if status == promptdomain.PromptStatusPublished {
		now := time.Now()
		entity.PublishedAt = &now
		entity.LatestVersionNo = 1
	}
	if err := s.prompts.Create(ctx, entity); err != nil {
		return SaveOutput{}, err
	}
	if status != promptdomain.PromptStatusPublished {
		entity.LatestVersionNo = 0
		if err := s.prompts.UpdateMetadata(ctx, entity.ID, repository.PromptMetadataUpdate{
			LatestVersionNo: &entity.LatestVersionNo,
		}); err != nil {
			return SaveOutput{}, fmt.Errorf("reset prompt version metadata: %w", err)
		}
	}
	relations, err := s.upsertPromptKeywords(ctx, input.UserID, entity.Topic, entity.ID, input.PositiveKeywords, input.NegativeKeywords)
	if err != nil {
		return SaveOutput{}, err
	}
	if err := s.keywords.ReplacePromptKeywords(ctx, entity.ID, relations); err != nil {
		s.logger.Warnw("replace prompt keywords failed", "promptID", entity.ID, "error", err)
	}
	if status == promptdomain.PromptStatusPublished {
		if err := s.recordPromptVersion(ctx, entity); err != nil {
			return SaveOutput{}, err
		}
	}
	return SaveOutput{PromptID: entity.ID, Status: entity.Status, Version: entity.LatestVersionNo}, nil
}

// updatePromptRecord 更新已有 Prompt，并在发布时生成新的版本快照。
func (s *Service) updatePromptRecord(ctx context.Context, input SaveInput, status string) (SaveOutput, error) {
	sanitizedTopic := normalizeMixedLanguageSpacing(strings.TrimSpace(input.Topic))
	entity, err := s.prompts.FindByID(ctx, input.UserID, input.PromptID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if existing, findErr := s.prompts.FindByUserAndTopic(ctx, input.UserID, sanitizedTopic); findErr == nil {
				entity = existing
			} else {
				return SaveOutput{}, fmt.Errorf("prompt not found")
			}
		} else {
			return SaveOutput{}, err
		}
	}
	encodedPos, err := s.marshalKeywordItems(input.PositiveKeywords)
	if err != nil {
		return SaveOutput{}, err
	}
	encodedNeg, err := s.marshalKeywordItems(input.NegativeKeywords)
	if err != nil {
		return SaveOutput{}, err
	}
	encodedTags, err := json.Marshal(input.Tags)
	if err != nil {
		return SaveOutput{}, fmt.Errorf("encode tags: %w", err)
	}
	entity.Topic = sanitizedTopic
	entity.Body = input.Body
	entity.Instructions = input.Instructions
	entity.PositiveKeywords = string(encodedPos)
	entity.NegativeKeywords = string(encodedNeg)
	entity.Model = strings.TrimSpace(input.Model)
	entity.Status = status
	entity.Tags = string(encodedTags)
	if status == promptdomain.PromptStatusPublished {
		currentVersion := entity.LatestVersionNo
		if entity.ID != 0 {
			maxVersion, err := s.prompts.MaxVersionNo(ctx, entity.ID)
			if err != nil {
				return SaveOutput{}, fmt.Errorf("load prompt version: %w", err)
			}
			if maxVersion > currentVersion {
				currentVersion = maxVersion
			}
		}
		if currentVersion < 0 {
			currentVersion = 0
		}
		entity.LatestVersionNo = currentVersion + 1
		now := time.Now()
		entity.PublishedAt = &now
	}
	if err := s.prompts.Update(ctx, entity); err != nil {
		return SaveOutput{}, err
	}
	relations, err := s.upsertPromptKeywords(ctx, input.UserID, entity.Topic, entity.ID, input.PositiveKeywords, input.NegativeKeywords)
	if err != nil {
		return SaveOutput{}, err
	}
	if err := s.keywords.ReplacePromptKeywords(ctx, entity.ID, relations); err != nil {
		s.logger.Warnw("replace prompt keywords failed", "promptID", entity.ID, "error", err)
	}
	if status == promptdomain.PromptStatusPublished {
		if err := s.recordPromptVersion(ctx, entity); err != nil {
			return SaveOutput{}, err
		}
		if s.versionKeepLimit > 0 {
			if err := s.prompts.DeleteOldVersions(ctx, entity.ID, s.versionKeepLimit); err != nil {
				s.logger.Warnw("delete old versions failed", "promptID", entity.ID, "error", err)
			}
		}
	}
	return SaveOutput{PromptID: entity.ID, Status: entity.Status, Version: entity.LatestVersionNo}, nil
}

// upsertPromptKeywords 确保关键词字典与 Prompt 关联表保持同步。
func (s *Service) upsertPromptKeywords(ctx context.Context, userID uint, topic string, promptID uint, positive, negative []KeywordItem) ([]promptdomain.PromptKeyword, error) {
	relations := make([]promptdomain.PromptKeyword, 0, len(positive)+len(negative))
	for _, item := range positive {
		stored, err := s.keywords.Upsert(ctx, toKeywordEntity(userID, topic, item))
		if err != nil {
			return nil, err
		}
		relations = append(relations, promptdomain.PromptKeyword{
			PromptID:  promptID,
			KeywordID: stored.ID,
			Relation:  promptdomain.KeywordPolarityPositive,
		})
	}
	for _, item := range negative {
		stored, err := s.keywords.Upsert(ctx, toKeywordEntity(userID, topic, item))
		if err != nil {
			return nil, err
		}
		relations = append(relations, promptdomain.PromptKeyword{
			PromptID:  promptID,
			KeywordID: stored.ID,
			Relation:  promptdomain.KeywordPolarityNegative,
		})
	}
	return relations, nil
}

// validatePublishInput 会在发布前检查必要字段，缺失时返回可读的错误提示。
func (s *Service) validatePublishInput(input SaveInput) error {
	missing := make([]string, 0, 6)
	if strings.TrimSpace(input.Topic) == "" {
		missing = append(missing, "主题")
	}
	if strings.TrimSpace(input.Body) == "" {
		missing = append(missing, "正文")
	}
	if strings.TrimSpace(input.Instructions) == "" {
		missing = append(missing, "补充要求")
	}
	if strings.TrimSpace(input.Model) == "" {
		missing = append(missing, "模型")
	}
	if len(input.PositiveKeywords) == 0 {
		missing = append(missing, "正向关键词")
	}
	if len(input.NegativeKeywords) == 0 {
		missing = append(missing, "负向关键词")
	}
	if len(input.Tags) == 0 {
		missing = append(missing, "标签")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("发布失败：缺少%s", strings.Join(missing, "、"))
}

// recordPromptVersion 写入 Prompt 的历史版本，便于后续回滚。
func (s *Service) recordPromptVersion(ctx context.Context, prompt *promptdomain.Prompt) error {
	version := &promptdomain.PromptVersion{
		PromptID:         prompt.ID,
		VersionNo:        prompt.LatestVersionNo,
		Body:             prompt.Body,
		Instructions:     prompt.Instructions,
		PositiveKeywords: prompt.PositiveKeywords,
		NegativeKeywords: prompt.NegativeKeywords,
		Model:            prompt.Model,
	}
	return s.prompts.CreateVersion(ctx, version)
}

type keywordPayload struct {
	Word   string `json:"word"`
	Weight int    `json:"weight"`
}

type interpretationPayload struct {
	Topic        string
	Positive     []keywordPayload
	Negative     []keywordPayload
	Confidence   float64
	Instructions string
	Tags         []string
}

// rawInterpretationPayload 用于兼容模型返回的多种 instructions 表达形式（字符串或数组）。
type rawInterpretationPayload struct {
	Topic        string           `json:"topic"`
	Positive     []keywordPayload `json:"positive_keywords"`
	Negative     []keywordPayload `json:"negative_keywords"`
	Confidence   float64          `json:"confidence"`
	Instructions interface{}      `json:"instructions"`
	Tags         interface{}      `json:"tags"`
}

// auditPayload 描述模型审核返回的结构化结果。
type auditPayload struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

// parseAuditPayload 解析内容审核模型返回的 JSON 结果，判断是否放行。
func parseAuditPayload(resp modeldomain.ChatCompletionResponse) (auditPayload, error) {
	if len(resp.Choices) == 0 {
		return auditPayload{}, errors.New("content audit returned no choices")
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if content == "" {
		return auditPayload{}, errors.New("content audit returned empty message")
	}
	var payload auditPayload
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return auditPayload{}, fmt.Errorf("decode audit response: %w", err)
	}
	return payload, nil
}

func parseInterpretationPayload(resp modeldomain.ChatCompletionResponse) (interpretationPayload, error) {
	if len(resp.Choices) == 0 {
		return interpretationPayload{}, errors.New("model returned no choices")
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if content == "" {
		return interpretationPayload{}, errors.New("model returned empty content")
	}
	var raw rawInterpretationPayload
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return interpretationPayload{}, fmt.Errorf("decode interpretation response: %w", err)
	}
	result := interpretationPayload{
		Topic:        strings.TrimSpace(raw.Topic),
		Positive:     normalizeKeywordPayloadSlice(raw.Positive),
		Negative:     normalizeKeywordPayloadSlice(raw.Negative),
		Confidence:   raw.Confidence,
		Instructions: normalizeInstructions(raw.Instructions),
		Tags:         normalizeTagPayload(raw.Tags),
	}
	return result, nil
}

type augmentPayload struct {
	Positive []keywordPayload `json:"positive_keywords"`
	Negative []keywordPayload `json:"negative_keywords"`
}

func parseAugmentPayload(resp modeldomain.ChatCompletionResponse) (augmentPayload, error) {
	if len(resp.Choices) == 0 {
		return augmentPayload{}, errors.New("model returned no choices")
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if content == "" {
		return augmentPayload{}, errors.New("model returned empty content")
	}
	var payload augmentPayload
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return augmentPayload{}, fmt.Errorf("decode augment response: %w", err)
	}
	payload.Positive = normalizeKeywordPayloadSlice(payload.Positive)
	payload.Negative = normalizeKeywordPayloadSlice(payload.Negative)
	return payload, nil
}

// normalizeKeywordPayloadSlice 对模型返回的关键词 payload 进行清洗，补齐权重并去重。
func normalizeKeywordPayloadSlice(items []keywordPayload) []keywordPayload {
	out := make([]keywordPayload, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		word := strings.TrimSpace(item.Word)
		if word == "" {
			continue
		}
		key := strings.ToLower(word)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		weight := clampWeight(item.Weight)
		out = append(out, keywordPayload{
			Word:   word,
			Weight: weight,
		})
	}
	return out
}

// normalizeInstructions 将模型返回的补充要求兼容为统一字符串格式。
func normalizeInstructions(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []string:
		// 部分模型会以字符串数组返回补充要求，这里拼接成单个字符串写回状态。
		return joinInstructions(v)
	case []interface{}:
		items := make([]string, 0, len(v))
		for _, item := range v {
			switch elem := item.(type) {
			case string:
				items = append(items, elem)
			case fmt.Stringer:
				items = append(items, elem.String())
			default:
				items = append(items, fmt.Sprintf("%v", elem))
			}
		}
		return joinInstructions(items)
	default:
		return ""
	}
}

// joinInstructions 使用中文分号拼接多条补充要求。
func joinInstructions(items []string) string {
	var builder strings.Builder
	for _, item := range items {
		text := strings.TrimSpace(item)
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("；")
		}
		builder.WriteString(text)
	}
	return builder.String()
}

// normalizeAuditReason 统一清洗内容审核的拒绝理由，保证提示语友好。
func normalizeAuditReason(reason string) string {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return "内容包含敏感信息，请修改后再试"
	}
	return trimmed
}

// normalizeTagPayload 将模型返回的标签字段整理为去重后的字符串列表。
func normalizeTagPayload(value interface{}) []string {
	if value == nil {
		return []string{}
	}
	var candidates []string
	switch v := value.(type) {
	case []string:
		candidates = v
	case []interface{}:
		for _, item := range v {
			str, ok := item.(string)
			if !ok {
				continue
			}
			candidates = append(candidates, str)
		}
	case string:
		candidates = append(candidates, v)
	default:
		return []string{}
	}
	result := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

// clampWeight 用于清洗模型或前端传入的权重，避免写入异常值。
func clampWeight(value int) int {
	if value < minKeywordWeight {
		return minKeywordWeight
	}
	if value > maxKeywordWeight {
		return maxKeywordWeight
	}
	return value
}

func extractPromptText(resp modeldomain.ChatCompletionResponse) string {
	if len(resp.Choices) == 0 {
		return ""
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content)
}

func (s *Service) marshalKeywordItems(items []KeywordItem) ([]byte, error) {
	payload := make([]promptdomain.PromptKeywordItem, 0, len(items))
	for _, item := range items {
		word := s.clampKeywordWord(item.Word)
		payload = append(payload, promptdomain.PromptKeywordItem{
			KeywordID: item.KeywordID,
			Word:      word,
			Source:    sourceFallback(item.Source),
			Weight:    clampWeight(item.Weight),
		})
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode keywords: %w", err)
	}
	return data, nil
}

// normalizeStatus 结合前端状态与 publish 标记推导最终的存储状态。
func normalizeStatus(status string, publish bool) string {
	if publish {
		return promptdomain.PromptStatusPublished
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case promptdomain.PromptStatusPublished:
		return promptdomain.PromptStatusPublished
	case promptdomain.PromptStatusArchived:
		return promptdomain.PromptStatusArchived
	default:
		return promptdomain.PromptStatusDraft
	}
}

// keywordSet 用于关键词去重，确保正负面不重复。
type keywordSet struct {
	seen map[string]struct{}
}

func newKeywordSet() *keywordSet {
	return &keywordSet{seen: make(map[string]struct{})}
}

func (s *keywordSet) add(item KeywordItem) bool {
	word := strings.TrimSpace(item.Word)
	if word == "" {
		return false
	}
	key := strings.ToLower(word)
	if _, ok := s.seen[key]; ok {
		return false
	}
	s.seen[key] = struct{}{}
	return true
}

func trimToRuneLength(input string, limit int) string {
	if limit <= 0 {
		return input
	}
	runes := []rune(input)
	if len(runes) <= limit {
		return input
	}
	return string(runes[:limit])
}

// runeCategory 表示用于判断分隔符插入的字符类别。
type runeCategory int

const (
	runeCategoryUnknown runeCategory = iota
	runeCategoryHan
	runeCategoryAlphaNumeric
	runeCategorySpace
	runeCategoryOther
)

// normalizeMixedLanguageSpacing 在中英文字符之间插入空格，便于后续分词与检索。
func normalizeMixedLanguageSpacing(input string) string {
	if input == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(input) + len(input)/2)
	prevCategory := runeCategoryUnknown
	hasPrev := false
	for _, r := range input {
		currCategory := classifyRuneForSpacing(r)
		if hasPrev && shouldInsertSpacing(prevCategory, currCategory) {
			builder.WriteRune(' ')
		}
		builder.WriteRune(r)
		if currCategory == runeCategorySpace {
			hasPrev = false
			prevCategory = runeCategoryUnknown
			continue
		}
		if currCategory == runeCategoryOther {
			prevCategory = runeCategoryOther
			hasPrev = false
			continue
		}
		prevCategory = currCategory
		hasPrev = true
	}
	return builder.String()
}

// classifyRuneForSpacing 将字符分类，以便判断是否需要插入空格。
func classifyRuneForSpacing(r rune) runeCategory {
	switch {
	case r == ' ' || unicode.IsSpace(r):
		return runeCategorySpace
	case unicode.Is(unicode.Han, r):
		return runeCategoryHan
	case r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
		return runeCategoryAlphaNumeric
	case unicode.IsLetter(r) || unicode.IsDigit(r):
		return runeCategoryAlphaNumeric
	default:
		return runeCategoryOther
	}
}

// shouldInsertSpacing 判断前后两个字符类别是否需要插入空格。
func shouldInsertSpacing(prev, curr runeCategory) bool {
	if prev == runeCategorySpace || curr == runeCategorySpace {
		return false
	}
	if prev == runeCategoryUnknown || curr == runeCategoryUnknown {
		return false
	}
	if prev == runeCategoryOther || curr == runeCategoryOther {
		return false
	}
	return (prev == runeCategoryHan && curr == runeCategoryAlphaNumeric) ||
		(prev == runeCategoryAlphaNumeric && curr == runeCategoryHan)
}

// clampKeywordWord 裁剪单个关键词至最大长度。
func (s *Service) clampKeywordWord(word string) string {
	trimmed := strings.TrimSpace(word)
	if s.keywordMaxLength > 0 && len([]rune(trimmed)) > s.keywordMaxLength {
		trimmed = trimToRuneLength(trimmed, s.keywordMaxLength)
	}
	return trimmed
}

// clampKeywordList 裁剪关键词列表中的每个词至最大长度。
func (s *Service) clampKeywordList(items []KeywordItem) []KeywordItem {
	for idx := range items {
		items[idx].Word = s.clampKeywordWord(items[idx].Word)
	}
	return items
}

// truncateTags 裁剪标签列表至最大数量。
func (s *Service) clampTagValue(tag string) string {
	trimmed := strings.TrimSpace(tag)
	if trimmed == "" {
		return ""
	}
	trimmed = normalizeMixedLanguageSpacing(trimmed)
	if s.tagMaxLength > 0 && len([]rune(trimmed)) > s.tagMaxLength {
		trimmed = trimToRuneLength(trimmed, s.tagMaxLength)
	}
	return trimmed
}

// buildAuditRequest 根据业务阶段构造内容审核提示词，要求模型返回允许与否。
func buildAuditRequest(stage auditStage, content string) modeldomain.ChatCompletionRequest {
	stageHint := "解析前的用户输入"
	switch stage {
	case auditStageGenerateOutput:
		stageHint = "模型生成的 Prompt 文本"
	case auditStageCommentBody:
		stageHint = "用户评论内容"
	}
	system := "你是一名内容审核助手，需要识别文本中是否包含黄赌毒、暴力、仇恨、违法或其他违反政策的内容。输出必须严格遵循 JSON 结构。"
	builder := &strings.Builder{}
	fmt.Fprintf(builder, "审核阶段：%s\n待审核内容：\n%s\n\n请按照以下格式返回：{\"allowed\":true/false,\"reason\":\"若不允许，请说明原因\"}。", stageHint, content)
	return modeldomain.ChatCompletionRequest{
		Messages: []modeldomain.ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: builder.String()},
		},
		ResponseFormat: map[string]any{"type": "json_object"},
	}
}

// buildInterpretationRequest 拼装解析自然语言描述所需的模型请求。
func buildInterpretationRequest(description, language string) modeldomain.ChatCompletionRequest {
	lang := languageOrDefault(language)
	system := "你是一名 Prompt 主题解析助手，负责从用户的自然语言意图中提炼主题、补充要求以及关键词。请始终返回结构化 JSON。"
	user := fmt.Sprintf(
		"目标语言：%s\n请从以下描述中提炼一个主题，拆分 3~6 个正向关键词与 1~4 个负向关键词，并总结 1~2 条补充要求。每个关键词需返回 0~5 的整数权重，表示与主题的相关度（0 为几乎无关，5 为强相关）。此外，请额外提供一个与主题相关的标签数组，帮助快速检索。输出 JSON（保持字段命名一致）：\n"+
			"{\"topic\":\"主题名称\",\"instructions\":\"补充要求\",\"positive_keywords\":[{\"word\":\"关键词\",\"weight\":0-5}],\"negative_keywords\":[{\"word\":\"关键词\",\"weight\":0-5}],\"tags\":[\"标签\"],\"confidence\":0.0-1.0}\n"+
			"描述：%s",
		lang, description,
	)
	return modeldomain.ChatCompletionRequest{
		Model: modeldomain.ChatCompletionRequest{}.Model,
		Messages: []modeldomain.ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		ResponseFormat: map[string]any{"type": "json_object"},
	}
}

// buildAugmentRequest 构建模型补充关键词的提示词上下文。
func buildAugmentRequest(input AugmentInput) modeldomain.ChatCompletionRequest {
	lang := languageOrDefault(input.Language)
	system := "你是一名关键词扩写助手，需要补充与主题相关的关键词，并避免重复已有词汇。"
	builder := &strings.Builder{}
	fmt.Fprintf(builder, "目标语言：%s\n主题：%s\n", lang, input.Topic)
	fmt.Fprintf(builder, "已有正向关键词：%s\n", joinKeywordWords(input.ExistingPositive))
	fmt.Fprintf(builder, "已有负向关键词：%s\n", joinKeywordWords(input.ExistingNegative))
	fmt.Fprintf(builder, "请补充不超过 %d 个正向关键词与 %d 个负向关键词，保持 JSON 输出，并为每个关键词给出 0~5 的整数权重（5 表示与主题高度相关）：\n"+
		"{\"positive_keywords\":[{\"word\":\"词汇\",\"weight\":0-5}],\"negative_keywords\":[{\"word\":\"词汇\",\"weight\":0-5}]}",
		defaultIfZero(input.RequestedPositive, 5),
		defaultIfZero(input.RequestedNegative, 3),
	)
	return modeldomain.ChatCompletionRequest{
		Messages: []modeldomain.ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: builder.String()},
		},
		ResponseFormat: map[string]any{"type": "json_object"},
	}
}

// buildGenerateRequest 依据主题与关键词生成最终 Prompt 的模型请求体。
func buildGenerateRequest(input GenerateInput) modeldomain.ChatCompletionRequest {
	lang := languageOrDefault(input.Language)
	system := "你是一名 Prompt 工程师，需要根据给定主题与关键词生成高质量的提示词，帮助大模型完成任务。"
	builder := &strings.Builder{}
	fmt.Fprintf(builder, "请面向 %s 输出一个完整 Prompt，仅返回最终 Prompt 文本。\n", lang)
	fmt.Fprintf(builder, "主题：%s\n", input.Topic)
	if desc := strings.TrimSpace(input.Description); desc != "" {
		fmt.Fprintf(builder, "用户需求描述：%s\n", desc)
	}
	fmt.Fprintf(builder, "正向关键词：%s\n", joinKeywordWords(input.PositiveKeywords))
	if len(input.NegativeKeywords) > 0 {
		fmt.Fprintf(builder, "负向关键词：%s\n", joinKeywordWords(input.NegativeKeywords))
	}
	fmt.Fprintf(builder, "请优先覆盖相关度较高（接近 %d）的正向关键词，并避免引入负向关键词。\n", maxKeywordWeight)
	if strings.TrimSpace(input.Instructions) != "" {
		fmt.Fprintf(builder, "补充要求：%s\n", input.Instructions)
	}
	if strings.TrimSpace(input.Tone) != "" {
		fmt.Fprintf(builder, "语气偏好：%s\n", input.Tone)
	}
	if existing := strings.TrimSpace(input.ExistingBody); existing != "" {
		fmt.Fprintf(builder, "以下是当前的 Prompt 草稿，请在保留核心信息的前提下加以润色和补充：\n%s\n请基于这份草稿做定向优化，输出优化后的完整 Prompt 文本，而不是完全重写。\n", existing)
	}
	if input.IncludeKeywordRef {
		fmt.Fprintf(builder, "请在 Prompt 中自然融入这些关键词，而非简单罗列。")
	}
	return modeldomain.ChatCompletionRequest{
		Model:       strings.TrimSpace(input.ModelKey),
		Messages:    []modeldomain.ChatMessage{{Role: "system", Content: system}, {Role: "user", Content: builder.String()}},
		Temperature: input.Temperature,
		MaxTokens:   input.MaxTokens,
	}
}

// keywordItemsToDomain 将 KeywordItem 列表转换为持久化使用的 PromptKeywordItem。
func keywordItemsToDomain(items []KeywordItem) []promptdomain.PromptKeywordItem {
	if len(items) == 0 {
		return []promptdomain.PromptKeywordItem{}
	}
	result := make([]promptdomain.PromptKeywordItem, 0, len(items))
	for _, item := range items {
		result = append(result, promptdomain.PromptKeywordItem{
			KeywordID: item.KeywordID,
			Word:      item.Word,
			Source:    item.Source,
			Weight:    item.Weight,
		})
	}
	return result
}

// decodePromptKeywords 将数据库中的关键词 JSON 字符串解析为 KeywordItem 列表，并做基础归一化。
func decodePromptKeywords(raw string) []KeywordItem {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []KeywordItem{}
	}
	var items []KeywordItem
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return []KeywordItem{}
	}
	for idx := range items {
		items[idx].Word = strings.TrimSpace(items[idx].Word)
		items[idx].Source = sourceFallback(items[idx].Source)
		items[idx].Polarity = normalizePolarity(items[idx].Polarity)
		items[idx].Weight = clampWeight(items[idx].Weight)
	}
	return items
}

// normalizeTags 去除空白/重复标签，并校验数量是否超过限制。
func (s *Service) normalizeTags(tags []string) ([]string, error) {
	if len(tags) == 0 {
		return []string{}, nil
	}
	limit := s.tagLimit
	cleaned := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		trimmed := s.clampTagValue(tag)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		if limit > 0 && len(cleaned) >= limit {
			return nil, ErrTagLimitExceeded
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, trimmed)
	}
	return cleaned, nil
}

// truncateTags 在读取已有数据时做降噪，防止历史数据中的标签超出上限。
func (s *Service) truncateTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}
	limit := s.tagLimit
	cleaned := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		trimmed := s.clampTagValue(tag)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, trimmed)
		if limit > 0 && len(cleaned) >= limit {
			break
		}
	}
	return cleaned
}

// extractTagsFromAttributes 从 Redis workspace attributes 中解析标签字符串并转换为切片。
func extractTagsFromAttributes(attrs map[string]string) []string {
	if len(attrs) == 0 {
		return nil
	}
	raw := strings.TrimSpace(attrs[workspaceAttrTags])
	if raw == "" {
		return nil
	}
	return decodeTags(raw)
}

func extractInstructionsFromAttributes(attrs map[string]string) string {
	// extractInstructionsFromAttributes 从 Workspace 的 attributes 中解析补充要求字符串。
	if len(attrs) == 0 {
		return ""
	}
	return strings.TrimSpace(attrs[workspaceAttrInstructions])
}

// encodeTagsAttribute 将标签数组编码为 JSON 字符串，写入 workspace attributes。
func encodeTagsAttribute(tags []string) string {
	raw, err := json.Marshal(tags)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

// decodeTags 兼容 JSON/逗号两种格式，输出去除空白的标签列表。
func decodeTags(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}
	}
	var tags []string
	if err := json.Unmarshal([]byte(trimmed), &tags); err == nil {
		cleaned := make([]string, 0, len(tags))
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				cleaned = append(cleaned, tag)
			}
		}
		return cleaned
	}
	parts := strings.Split(trimmed, ",")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return cleaned
}

// normaliseExportDir 规范化导出目录路径，处理 ~ 前缀与相对路径。
func normaliseExportDir(path string) (string, error) {
	expanded, err := expandHomePath(path)
	if err != nil {
		return "", err
	}
	if expanded == "" {
		expanded = DefaultPromptExportDir
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// expandHomePath 将以 ~ 开头的路径展开为用户主目录。
func expandHomePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", nil
	}
	if !strings.HasPrefix(trimmed, "~") {
		return trimmed, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	remainder := strings.TrimPrefix(trimmed, "~")
	remainder = strings.TrimLeft(remainder, "/\\")
	if remainder == "" {
		return home, nil
	}
	return filepath.Join(home, remainder), nil
}

// languageOrDefault 若未指定语言则返回中文，供模型提示使用。
func languageOrDefault(lang string) string {
	trimmed := strings.TrimSpace(lang)
	if trimmed == "" {
		return "中文"
	}
	return trimmed
}

// joinKeywordWords 将关键词渲染成带权重的提示字符串。
func joinKeywordWords(items []KeywordItem) string {
	if len(items) == 0 {
		return "（无）"
	}
	words := make([]string, 0, len(items))
	for _, item := range items {
		word := strings.TrimSpace(item.Word)
		if word == "" {
			continue
		}
		weight := clampWeight(item.Weight)
		words = append(words, fmt.Sprintf("%s（相关度 %d/%d）", word, weight, maxKeywordWeight))
	}
	if len(words) == 0 {
		return "（无）"
	}
	return strings.Join(words, "、")
}

// defaultIfZero 在请求参数缺省时使用备用值。
func defaultIfZero(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
