package prompt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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
	UpdateDraftBody(ctx context.Context, userID uint, token, body string) error
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
	prompts   *repository.PromptRepository
	keywords  *repository.KeywordRepository
	model     ModelInvoker
	workspace WorkspaceStore
	queue     PersistenceQueue
	logger    *zap.SugaredLogger
}

const (
	defaultQueuePollInterval     = 2 * time.Second
	defaultModelInvokeTimeout    = 35 * time.Second
	defaultWorkspaceWriteTimeout = 5 * time.Second
)

// NewService 构建 Service。
func NewService(prompts *repository.PromptRepository, keywords *repository.KeywordRepository, model ModelInvoker, workspace WorkspaceStore, queue PersistenceQueue, logger *zap.SugaredLogger) *Service {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	return &Service{
		prompts:   prompts,
		keywords:  keywords,
		model:     model,
		workspace: workspace,
		queue:     queue,
		logger:    logger,
	}
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

// 给 Redis 写入操作准备一个不受请求取消影响、并且有 5 秒兜底超时的上下文。原理跟 modelInvocationContext 类似
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
		return InterpretOutput{}, fmt.Errorf("invoke model: %w", err)
	}

	payload, err := parseInterpretationPayload(resp)
	if err != nil {
		return InterpretOutput{}, err
	}

	result := InterpretOutput{
		Topic:      payload.Topic,
		Confidence: payload.Confidence,
	}
	if result.Topic == "" {
		return InterpretOutput{}, errors.New("model did not return topic")
	}
	normalized := newKeywordSet()
	for _, word := range payload.Positive {
		item := KeywordItem{
			Word:     word,
			Source:   promptdomain.KeywordSourceModel,
			Polarity: promptdomain.KeywordPolarityPositive,
		}
		if normalized.add(item) {
			result.PositiveKeywords = append(result.PositiveKeywords, item)
		}
	}
	for _, word := range payload.Negative {
		item := KeywordItem{
			Word:     word,
			Source:   promptdomain.KeywordSourceModel,
			Polarity: promptdomain.KeywordPolarityNegative,
		}
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
			Positive:  toWorkspaceKeywords(result.PositiveKeywords),
			Negative:  toWorkspaceKeywords(result.NegativeKeywords),
			Version:   1,
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

	return result, nil
}

// AugmentKeywords 调用模型补充关键词，并返回真正新增的词条。
func (s *Service) AugmentKeywords(ctx context.Context, input AugmentInput) (AugmentOutput, error) {
	if strings.TrimSpace(input.Topic) == "" {
		return AugmentOutput{}, errors.New("topic is empty")
	}
	if strings.TrimSpace(input.ModelKey) == "" {
		return AugmentOutput{}, errors.New("model key is empty")
	}

	req := buildAugmentRequest(input)
	req.Model = strings.TrimSpace(input.ModelKey)
	modelCtx, cancel := s.modelInvocationContext(ctx)
	defer cancel()
	resp, err := s.model.InvokeChatCompletion(modelCtx, input.UserID, input.ModelKey, req)
	if err != nil {
		return AugmentOutput{}, fmt.Errorf("invoke model: %w", err)
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
	for _, word := range payload.Positive {
		item := KeywordItem{
			Word:     word,
			Source:   promptdomain.KeywordSourceModel,
			Polarity: promptdomain.KeywordPolarityPositive,
		}
		if existing.add(item) {
			output.Positive = append(output.Positive, item)
			if workspaceEnabled {
				workspaceNew = append(workspaceNew, promptdomain.WorkspaceKeyword{
					Word:     strings.TrimSpace(item.Word),
					Source:   sourceFallback(item.Source),
					Polarity: promptdomain.KeywordPolarityPositive,
				})
			} else {
				if _, err := s.keywords.Upsert(ctx, toKeywordEntity(input.UserID, input.Topic, item)); err != nil {
					s.logger.Warnw("upsert keyword failed", "topic", input.Topic, "word", item.Word, "error", err)
				}
			}
		}
	}
	for _, word := range payload.Negative {
		item := KeywordItem{
			Word:     word,
			Source:   promptdomain.KeywordSourceModel,
			Polarity: promptdomain.KeywordPolarityNegative,
		}
		if existing.add(item) {
			output.Negative = append(output.Negative, item)
			if workspaceEnabled {
				workspaceNew = append(workspaceNew, promptdomain.WorkspaceKeyword{
					Word:     strings.TrimSpace(item.Word),
					Source:   sourceFallback(item.Source),
					Polarity: promptdomain.KeywordPolarityNegative,
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
	word := strings.TrimSpace(input.Word)
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
	if s.workspace != nil && strings.TrimSpace(input.WorkspaceToken) != "" {
		workspaceKeyword := promptdomain.WorkspaceKeyword{
			Word:     word,
			Source:   source,
			Polarity: polarity,
		}
		storeCtx, cancel := s.workspaceContext(ctx)
		defer cancel()
		if err := s.workspace.MergeKeywords(storeCtx, input.UserID, strings.TrimSpace(input.WorkspaceToken), []promptdomain.WorkspaceKeyword{workspaceKeyword}); err != nil {
			s.logger.Warnw("merge manual keyword to workspace failed", "user_id", input.UserID, "token", input.WorkspaceToken, "word", word, "error", err)
		} else if err := s.workspace.Touch(storeCtx, input.UserID, strings.TrimSpace(input.WorkspaceToken)); err != nil {
			s.logger.Warnw("touch workspace failed", "user_id", input.UserID, "token", input.WorkspaceToken, "error", err)
		}
		return KeywordItem{
			KeywordID: 0,
			Word:      word,
			Source:    source,
			Polarity:  polarity,
		}, nil
	}

	entity := toKeywordEntity(input.UserID, input.Topic, KeywordItem{
		Word:     word,
		Source:   source,
		Polarity: polarity,
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
	}
	if input.PromptID != 0 {
		if err := s.keywords.AttachToPrompt(ctx, input.PromptID, stored.ID, relationByPolarity(item.Polarity)); err != nil {
			s.logger.Warnw("attach manual keyword failed", "promptID", input.PromptID, "keywordID", stored.ID, "error", err)
		}
	}
	return item, nil
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
	req := buildGenerateRequest(input)
	req.Model = strings.TrimSpace(input.ModelKey)
	start := time.Now()
	modelCtx, cancel := s.modelInvocationContext(ctx)
	defer cancel()
	resp, err := s.model.InvokeChatCompletion(modelCtx, input.UserID, input.ModelKey, req)
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("invoke model: %w", err)
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

// Save 保存 Prompt 草稿或发布版本。
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
				input.PositiveKeywords = keywordItemsFromWorkspace(snapshot.Positive)
			}
			if len(input.NegativeKeywords) == 0 {
				input.NegativeKeywords = keywordItemsFromWorkspace(snapshot.Negative)
			}
			if snapshot.PromptID != 0 && input.PromptID == 0 {
				input.PromptID = snapshot.PromptID
			}
			if strings.TrimSpace(input.Status) == "" && strings.TrimSpace(snapshot.Status) != "" {
				status = normalizeStatus(snapshot.Status, input.Publish)
			}
		}
	}

	if input.PromptID == 0 && strings.TrimSpace(input.Topic) != "" {
		if existing, err := s.prompts.FindByUserAndTopic(ctx, input.UserID, strings.TrimSpace(input.Topic)); err == nil {
			input.PromptID = existing.ID
		}
	}

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
	return &promptdomain.Keyword{
		UserID:   userID,
		Topic:    strings.TrimSpace(topic),
		Word:     strings.TrimSpace(item.Word),
		Polarity: normalizePolarity(item.Polarity),
		Source:   sourceFallback(item.Source),
		Language: "zh",
	}
}

func toWorkspaceKeywords(items []KeywordItem) []promptdomain.WorkspaceKeyword {
	result := make([]promptdomain.WorkspaceKeyword, 0, len(items))
	for _, item := range items {
		result = append(result, promptdomain.WorkspaceKeyword{
			Word:     strings.TrimSpace(item.Word),
			Source:   sourceFallback(item.Source),
			Polarity: normalizePolarity(item.Polarity),
			Weight:   0,
		})
	}
	return result
}

func keywordItemsFromWorkspace(items []promptdomain.WorkspaceKeyword) []KeywordItem {
	result := make([]KeywordItem, 0, len(items))
	for _, item := range items {
		result = append(result, KeywordItem{
			Word:     strings.TrimSpace(item.Word),
			Source:   sourceFallback(item.Source),
			Polarity: normalizePolarity(item.Polarity),
		})
	}
	return result
}

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

func relationByPolarity(polarity string) string {
	if normalizePolarity(polarity) == promptdomain.KeywordPolarityNegative {
		return promptdomain.KeywordPolarityNegative
	}
	return promptdomain.KeywordPolarityPositive
}

func sourceFallback(source string) string {
	if source == "" {
		return promptdomain.KeywordSourceManual
	}
	return source
}

func (s *Service) persistKeywords(ctx context.Context, userID uint, topic string, items []KeywordItem) {
	for _, item := range items {
		if _, err := s.keywords.Upsert(ctx, toKeywordEntity(userID, topic, item)); err != nil {
			s.logger.Warnw("upsert keyword failed", "topic", topic, "word", item.Word, "error", err)
		}
	}
}

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
		Model:            firstNonEmpty(task.Model, snapshot.ModelKey),
		Status:           task.Status,
		PositiveKeywords: keywordItemsFromWorkspace(snapshot.Positive),
		NegativeKeywords: keywordItemsFromWorkspace(snapshot.Negative),
		Tags:             task.Tags,
		Publish:          task.Publish,
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
	result, err := s.persistPrompt(ctx, input, status, action)
	if err != nil {
		return fmt.Errorf("persist prompt: %w", err)
	}
	metaCtx, cancelMeta := s.workspaceContext(ctx)
	if err := s.workspace.SetPromptMeta(metaCtx, task.UserID, task.WorkspaceToken, result.PromptID, status); err != nil {
		s.logger.Warnw("set workspace meta failed", "task_id", task.TaskID, "token", task.WorkspaceToken, "error", err)
	}
	cancelMeta()
	s.logger.Infow("prompt persisted", "task_id", task.TaskID, "prompt_id", result.PromptID, "user_id", task.UserID, "publish", task.Publish)
	return nil
}

func (s *Service) persistPrompt(ctx context.Context, input SaveInput, status, action string) (SaveOutput, error) {
	if strings.TrimSpace(input.Topic) == "" {
		return SaveOutput{}, errors.New("topic required")
	}
	if strings.TrimSpace(input.Body) == "" {
		return SaveOutput{}, errors.New("body required")
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

func (s *Service) createPromptRecord(ctx context.Context, input SaveInput, status string) (SaveOutput, error) {
	encodedPos, err := marshalKeywordItems(input.PositiveKeywords)
	if err != nil {
		return SaveOutput{}, err
	}
	encodedNeg, err := marshalKeywordItems(input.NegativeKeywords)
	if err != nil {
		return SaveOutput{}, err
	}
	encodedTags, err := json.Marshal(input.Tags)
	if err != nil {
		return SaveOutput{}, fmt.Errorf("encode tags: %w", err)
	}
	entity := &promptdomain.Prompt{
		UserID:           input.UserID,
		Topic:            strings.TrimSpace(input.Topic),
		Body:             input.Body,
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

func (s *Service) updatePromptRecord(ctx context.Context, input SaveInput, status string) (SaveOutput, error) {
	entity, err := s.prompts.FindByID(ctx, input.UserID, input.PromptID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if existing, findErr := s.prompts.FindByUserAndTopic(ctx, input.UserID, strings.TrimSpace(input.Topic)); findErr == nil {
				entity = existing
			} else {
				return SaveOutput{}, fmt.Errorf("prompt not found")
			}
		} else {
			return SaveOutput{}, err
		}
	}
	encodedPos, err := marshalKeywordItems(input.PositiveKeywords)
	if err != nil {
		return SaveOutput{}, err
	}
	encodedNeg, err := marshalKeywordItems(input.NegativeKeywords)
	if err != nil {
		return SaveOutput{}, err
	}
	encodedTags, err := json.Marshal(input.Tags)
	if err != nil {
		return SaveOutput{}, fmt.Errorf("encode tags: %w", err)
	}
	wasPublished := entity.Status == promptdomain.PromptStatusPublished
	entity.Topic = strings.TrimSpace(input.Topic)
	entity.Body = input.Body
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
		if err := s.prompts.DeleteOldVersions(ctx, entity.ID, 3); err != nil {
			s.logger.Warnw("delete old versions failed", "promptID", entity.ID, "error", err)
		}
	}
	return SaveOutput{PromptID: entity.ID, Status: entity.Status, Version: entity.LatestVersionNo}, nil
}

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

func (s *Service) recordPromptVersion(ctx context.Context, prompt *promptdomain.Prompt) error {
	version := &promptdomain.PromptVersion{
		PromptID:         prompt.ID,
		VersionNo:        prompt.LatestVersionNo,
		Body:             prompt.Body,
		PositiveKeywords: prompt.PositiveKeywords,
		NegativeKeywords: prompt.NegativeKeywords,
		Model:            prompt.Model,
	}
	return s.prompts.CreateVersion(ctx, version)
}

type interpretationPayload struct {
	Topic      string   `json:"topic"`
	Positive   []string `json:"positive_keywords"`
	Negative   []string `json:"negative_keywords"`
	Confidence float64  `json:"confidence"`
}

func parseInterpretationPayload(resp modeldomain.ChatCompletionResponse) (interpretationPayload, error) {
	if len(resp.Choices) == 0 {
		return interpretationPayload{}, errors.New("model returned no choices")
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if content == "" {
		return interpretationPayload{}, errors.New("model returned empty content")
	}
	var payload interpretationPayload
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return interpretationPayload{}, fmt.Errorf("decode model response: %w", err)
	}
	payload.Topic = strings.TrimSpace(payload.Topic)
	payload.Positive = normalizeWordSlice(payload.Positive)
	payload.Negative = normalizeWordSlice(payload.Negative)
	return payload, nil
}

type augmentPayload struct {
	Positive []string `json:"positive_keywords"`
	Negative []string `json:"negative_keywords"`
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
		return augmentPayload{}, fmt.Errorf("decode model response: %w", err)
	}
	payload.Positive = normalizeWordSlice(payload.Positive)
	payload.Negative = normalizeWordSlice(payload.Negative)
	return payload, nil
}

func normalizeWordSlice(words []string) []string {
	out := make([]string, 0, len(words))
	seen := map[string]struct{}{}
	for _, word := range words {
		trimmed := strings.TrimSpace(word)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func extractPromptText(resp modeldomain.ChatCompletionResponse) string {
	if len(resp.Choices) == 0 {
		return ""
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content)
}

func marshalKeywordItems(items []KeywordItem) ([]byte, error) {
	payload := make([]promptdomain.PromptKeywordItem, 0, len(items))
	for _, item := range items {
		payload = append(payload, promptdomain.PromptKeywordItem{
			KeywordID: item.KeywordID,
			Word:      strings.TrimSpace(item.Word),
			Source:    sourceFallback(item.Source),
		})
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode keywords: %w", err)
	}
	return data, nil
}

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

func buildInterpretationRequest(description, language string) modeldomain.ChatCompletionRequest {
	lang := languageOrDefault(language)
	system := "你是一名 Prompt 主题解析助手，负责从用户的自然语言意图中提炼主题与关键词。请始终返回结构化 JSON。"
	user := fmt.Sprintf(
		"目标语言：%s\n请从以下描述中提炼一个主题，拆分 3~6 个正向关键词与 1~4 个负向关键词，输出 JSON：\n"+
			"{\"topic\":\"主题名称\",\"positive_keywords\":[\"关键词\"],\"negative_keywords\":[\"关键词\"],\"confidence\":0.0~1.0}\n"+
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

func buildAugmentRequest(input AugmentInput) modeldomain.ChatCompletionRequest {
	lang := languageOrDefault(input.Language)
	system := "你是一名关键词扩写助手，需要补充与主题相关的关键词，并避免重复已有词汇。"
	builder := &strings.Builder{}
	fmt.Fprintf(builder, "目标语言：%s\n主题：%s\n", lang, input.Topic)
	fmt.Fprintf(builder, "已有正向关键词：%s\n", joinKeywordWords(input.ExistingPositive))
	fmt.Fprintf(builder, "已有负向关键词：%s\n", joinKeywordWords(input.ExistingNegative))
	fmt.Fprintf(builder, "请补充不超过 %d 个正向关键词与 %d 个负向关键词，保持 JSON 输出：\n"+
		"{\"positive_keywords\":[\"词汇\"],\"negative_keywords\":[\"词汇\"]}",
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

func languageOrDefault(lang string) string {
	trimmed := strings.TrimSpace(lang)
	if trimmed == "" {
		return "中文"
	}
	return trimmed
}

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
		words = append(words, word)
	}
	if len(words) == 0 {
		return "（无）"
	}
	return strings.Join(words, "、")
}

func defaultIfZero(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
