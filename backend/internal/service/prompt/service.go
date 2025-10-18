package prompt

import (
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
	"electron-go-app/backend/internal/repository"

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

// DefaultVersionRetentionLimit 默认保留的 Prompt 历史版本数量。
const DefaultVersionRetentionLimit = 5

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
}

// NewServiceWithConfig 构建 Service，并允许自定义分页等配置。
func NewServiceWithConfig(prompts *repository.PromptRepository, keywords *repository.KeywordRepository, model ModelInvoker, workspace WorkspaceStore, queue PersistenceQueue, logger *zap.SugaredLogger, cfg Config) *Service {
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
		cfg.DefaultListPageSize = 20
	}
	if cfg.MaxListPageSize <= 0 {
		cfg.MaxListPageSize = 100
	}
	if cfg.DefaultListPageSize > cfg.MaxListPageSize {
		cfg.DefaultListPageSize = cfg.MaxListPageSize
	}
	if cfg.VersionRetention <= 0 {
		cfg.VersionRetention = DefaultVersionRetentionLimit
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
	return &Service{
		prompts:             prompts,
		keywords:            keywords,
		model:               model,
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
	}
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
	UserID   uint
	Status   string
	Query    string
	Page     int
	PageSize int
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

// SaveInput 描述保存草稿或发布 Prompt 的参数。
type SaveInput struct {
	UserID           uint
	PromptID         uint
	Topic            string
	Body             string
	Instructions     string
	Model            string
	Status           string
	PositiveKeywords []KeywordItem
	NegativeKeywords []KeywordItem
	Tags             []string
	Publish          bool
	WorkspaceToken   string
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
		return InterpretOutput{}, errors.New("model key is empty")
	}
	req := buildInterpretationRequest(description, input.Language)
	req.Model = modelKey
	// 防止模型超时
	modelCtx, cancel := s.modelInvocationContext(ctx)
	defer cancel()
	resp, err := s.model.InvokeChatCompletion(modelCtx, input.UserID, modelKey, req)
	if err != nil {
		return InterpretOutput{}, fmt.Errorf("%w: %w", ErrModelInvocationFailed, err)
	}

	payload, err := parseInterpretationPayload(resp)
	if err != nil {
		return InterpretOutput{}, err
	}

	result := InterpretOutput{
		Topic:        payload.Topic,
		Confidence:   payload.Confidence,
		Instructions: payload.Instructions,
	}
	if result.Topic == "" {
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
			result.PositiveKeywords = append(result.PositiveKeywords, item)
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
			result.NegativeKeywords = append(result.NegativeKeywords, item)
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
			Positive:  toWorkspaceKeywords(result.PositiveKeywords, s.keywordLimit),
			Negative:  toWorkspaceKeywords(result.NegativeKeywords, s.keywordLimit),
			Version:   1,
		}
		if strings.TrimSpace(result.Instructions) != "" {
			workspaceSnapshot.Attributes = map[string]string{workspaceAttrInstructions: result.Instructions}
		}
		if token, err := s.workspace.CreateOrReplace(storeCtx, input.UserID, workspaceSnapshot); err != nil {
			s.logger.Warnw("store workspace snapshot failed", "user_id", input.UserID, "topic", payload.Topic, "error", err)
		} else {
			result.WorkspaceToken = token
		}
	} else {
		// 无 Redis 时保持旧行为，直接写入 MySQL 字典。
		s.persistKeywords(ctx, input.UserID, payload.Topic, append(result.PositiveKeywords, result.NegativeKeywords...))
	}

	if len(result.PositiveKeywords) == 0 {
		return InterpretOutput{}, errors.New("model did not return positive keywords")
	}
	if len(result.PositiveKeywords) > s.keywordLimit {
		result.PositiveKeywords = result.PositiveKeywords[:s.keywordLimit]
	}
	if len(result.NegativeKeywords) > s.keywordLimit {
		result.NegativeKeywords = result.NegativeKeywords[:s.keywordLimit]
	}

	return result, nil
}

// AugmentKeywords 调用模型补充关键词，并返回真正新增的词条，同时维持去重与上限控制。
func (s *Service) AugmentKeywords(ctx context.Context, input AugmentInput) (AugmentOutput, error) {
	if strings.TrimSpace(input.Topic) == "" {
		return AugmentOutput{}, errors.New("topic is empty")
	}
	if strings.TrimSpace(input.ModelKey) == "" {
		return AugmentOutput{}, errors.New("model key is empty")
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
	req.Model = strings.TrimSpace(input.ModelKey)
	modelCtx, cancel := s.modelInvocationContext(ctx)
	defer cancel()
	resp, err := s.model.InvokeChatCompletion(modelCtx, input.UserID, input.ModelKey, req)
	if err != nil {
		return AugmentOutput{}, fmt.Errorf("%w: %w", ErrModelInvocationFailed, err)
	}
	payload, err := parseAugmentPayload(resp)
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

// SyncWorkspaceKeywords 将前端的排序与权重同步到工作区缓存，保持 Redis 状态与 UI 一致。
// 若工作区 token 失效会返回错误，前端需重新获取最新数据。
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
	if strings.TrimSpace(input.ModelKey) == "" {
		return GenerateOutput{}, errors.New("model key is empty")
	}
	if len(input.PositiveKeywords) == 0 {
		return GenerateOutput{}, errors.New("positive keywords required")
	}
	if err := enforceKeywordLimit(s.keywordLimit, input.PositiveKeywords, input.NegativeKeywords); err != nil {
		return GenerateOutput{}, err
	}
	req := buildGenerateRequest(input)
	req.Model = strings.TrimSpace(input.ModelKey)
	start := time.Now()
	modelCtx, cancel := s.modelInvocationContext(ctx)
	defer cancel()
	resp, err := s.model.InvokeChatCompletion(modelCtx, input.UserID, input.ModelKey, req)
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("%w: %w", ErrModelInvocationFailed, err)
	}
	duration := time.Since(start)
	promptText := extractPromptText(resp)
	if promptText == "" {
		return GenerateOutput{}, errors.New("model returned empty prompt")
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
		Model:        resp.Model,
		Prompt:       promptText,
		Duration:     duration,
		Usage:        resp.Usage,
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
		UserID:           task.UserID,
		PromptID:         task.PromptID,
		Topic:            firstNonEmpty(task.Topic, snapshot.Topic),
		Body:             firstNonEmpty(task.Body, snapshot.DraftBody),
		Instructions:     firstNonEmpty(task.Instructions, extractInstructionsFromAttributes(snapshot.Attributes)),
		Model:            firstNonEmpty(task.Model, snapshot.ModelKey),
		Status:           task.Status,
		PositiveKeywords: s.keywordItemsFromWorkspace(snapshot.Positive),
		NegativeKeywords: s.keywordItemsFromWorkspace(snapshot.Negative),
		Tags:             task.Tags,
		Publish:          task.Publish,
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
func (s *Service) persistPrompt(ctx context.Context, input SaveInput, status, action string) (SaveOutput, error) {
	if strings.TrimSpace(input.Topic) == "" {
		return SaveOutput{}, errors.New("topic required")
	}
	if strings.TrimSpace(input.Body) == "" {
		return SaveOutput{}, errors.New("body required")
	}
	input.Instructions = strings.TrimSpace(input.Instructions)
	if err := enforceKeywordLimit(s.keywordLimit, input.PositiveKeywords, input.NegativeKeywords); err != nil {
		return SaveOutput{}, err
	}
	cleanedTags, err := s.normalizeTags(input.Tags)
	if err != nil {
		return SaveOutput{}, err
	}
	input.Tags = cleanedTags
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
	topic := normalizeMixedLanguageSpacing(strings.TrimSpace(input.Topic))
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
	wasPublished := entity.Status == promptdomain.PromptStatusPublished
	entity.Topic = sanitizedTopic
	entity.Body = input.Body
	entity.Instructions = input.Instructions
	entity.PositiveKeywords = string(encodedPos)
	entity.NegativeKeywords = string(encodedNeg)
	entity.Model = strings.TrimSpace(input.Model)
	entity.Status = status
	entity.Tags = string(encodedTags)
	if status == promptdomain.PromptStatusPublished {
		if wasPublished {
			entity.LatestVersionNo++
		} else {
			entity.LatestVersionNo = 1
		}
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
}

// rawInterpretationPayload 用于兼容模型返回的多种 instructions 表达形式（字符串或数组）。
type rawInterpretationPayload struct {
	Topic        string           `json:"topic"`
	Positive     []keywordPayload `json:"positive_keywords"`
	Negative     []keywordPayload `json:"negative_keywords"`
	Confidence   float64          `json:"confidence"`
	Instructions interface{}      `json:"instructions"`
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

// buildInterpretationRequest 拼装解析自然语言描述所需的模型请求。
func buildInterpretationRequest(description, language string) modeldomain.ChatCompletionRequest {
	lang := languageOrDefault(language)
	system := "你是一名 Prompt 主题解析助手，负责从用户的自然语言意图中提炼主题、补充要求以及关键词。请始终返回结构化 JSON。"
	user := fmt.Sprintf(
		"目标语言：%s\n请从以下描述中提炼一个主题，拆分 3~6 个正向关键词与 1~4 个负向关键词，并总结 1~2 条补充要求。每个关键词需返回 0~5 的整数权重，表示与主题的相关度（0 为几乎无关，5 为强相关）。输出 JSON（保持字段命名一致）：\n"+
			"{\"topic\":\"主题名称\",\"instructions\":\"补充要求\",\"positive_keywords\":[{\"word\":\"关键词\",\"weight\":0-5}],\"negative_keywords\":[{\"word\":\"关键词\",\"weight\":0-5}],\"confidence\":0.0-1.0}\n"+
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
