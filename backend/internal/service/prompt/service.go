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
	RemoveKeyword(ctx context.Context, userID uint, token, polarity, word string) error
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
	prompts      *repository.PromptRepository
	keywords     *repository.KeywordRepository
	model        ModelInvoker
	workspace    WorkspaceStore
	queue        PersistenceQueue
	logger       *zap.SugaredLogger
	keywordLimit int
}

const (
	defaultQueuePollInterval     = 2 * time.Second
	defaultModelInvokeTimeout    = 35 * time.Second
	defaultWorkspaceWriteTimeout = 5 * time.Second
)

// DefaultKeywordLimit 限制正/负向关键词数量的默认值。
const DefaultKeywordLimit = 10

const (
	// minKeywordWeight/maxKeywordWeight 定义关键词相关度（0~5）的允许区间。
	minKeywordWeight = 0
	maxKeywordWeight = 5
	// defaultKeywordWeight 用于 Interpret/Augment 未返回权重或手动录入时的兜底值。
	defaultKeywordWeight = 5
)

var (
	// ErrPositiveKeywordLimit 表示正向关键词数量超出上限。
	ErrPositiveKeywordLimit = errors.New("positive keywords exceed limit")
	// ErrNegativeKeywordLimit 表示负向关键词数量超出上限。
	ErrNegativeKeywordLimit = errors.New("negative keywords exceed limit")
	// ErrDuplicateKeyword 表示同极性的关键词已存在。
	ErrDuplicateKeyword = errors.New("keyword already exists")
)

// NewService 构建 Service。
func NewService(prompts *repository.PromptRepository, keywords *repository.KeywordRepository, model ModelInvoker, workspace WorkspaceStore, queue PersistenceQueue, logger *zap.SugaredLogger, keywordLimit int) *Service {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	if keywordLimit <= 0 {
		keywordLimit = DefaultKeywordLimit
	}
	return &Service{
		prompts:      prompts,
		keywords:     keywords,
		model:        model,
		workspace:    workspace,
		queue:        queue,
		logger:       logger,
		keywordLimit: keywordLimit,
	}
}

// KeywordLimit 暴露当前生效的关键词数量上限，供上层 Handler 使用。
func (s *Service) KeywordLimit() int {
	return s.keywordLimit
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
			workspaceSnapshot.Attributes = map[string]string{"instructions": result.Instructions}
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
	snapshot.Positive = workspaceKeywordsFromOrdered(input.Positive, s.keywordLimit)
	snapshot.Negative = workspaceKeywordsFromOrdered(input.Negative, s.keywordLimit)
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
				input.PositiveKeywords = keywordItemsFromWorkspace(snapshot.Positive, s.keywordLimit)
			}
			if len(input.NegativeKeywords) == 0 {
				input.NegativeKeywords = keywordItemsFromWorkspace(snapshot.Negative, s.keywordLimit)
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

	if err := enforceKeywordLimit(s.keywordLimit, input.PositiveKeywords, input.NegativeKeywords); err != nil {
		return SaveOutput{}, err
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
func workspaceKeywordsFromOrdered(items []KeywordItem, limit int) []promptdomain.WorkspaceKeyword {
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

// keywordItemsFromWorkspace 将工作区缓存的关键词还原为业务层结构体。
func keywordItemsFromWorkspace(items []promptdomain.WorkspaceKeyword, limit int) []KeywordItem {
	if limit <= 0 {
		limit = DefaultKeywordLimit
	}
	result := make([]KeywordItem, 0, len(items))
	for _, item := range items {
		result = append(result, KeywordItem{
			Word:     strings.TrimSpace(item.Word),
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
	for _, item := range items {
		if _, err := s.keywords.Upsert(ctx, toKeywordEntity(userID, topic, item)); err != nil {
			s.logger.Warnw("upsert keyword failed", "topic", topic, "word", item.Word, "error", err)
		}
	}
}

// processPersistenceTask 消费异步队列任务，将工作区内容持久化回数据库。
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
		PositiveKeywords: keywordItemsFromWorkspace(snapshot.Positive, s.keywordLimit),
		NegativeKeywords: keywordItemsFromWorkspace(snapshot.Negative, s.keywordLimit),
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
	if err := enforceKeywordLimit(s.keywordLimit, input.PositiveKeywords, input.NegativeKeywords); err != nil {
		return fmt.Errorf("keyword limit: %w", err)
	}
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

// persistPrompt 根据动作类型创建或更新 Prompt 主记录，同时保持关键词与版本一致。
func (s *Service) persistPrompt(ctx context.Context, input SaveInput, status, action string) (SaveOutput, error) {
	if strings.TrimSpace(input.Topic) == "" {
		return SaveOutput{}, errors.New("topic required")
	}
	if strings.TrimSpace(input.Body) == "" {
		return SaveOutput{}, errors.New("body required")
	}
	if err := enforceKeywordLimit(s.keywordLimit, input.PositiveKeywords, input.NegativeKeywords); err != nil {
		return SaveOutput{}, err
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

// updatePromptRecord 更新已有 Prompt，并在发布时生成新的版本快照。
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

func marshalKeywordItems(items []KeywordItem) ([]byte, error) {
	payload := make([]promptdomain.PromptKeywordItem, 0, len(items))
	for _, item := range items {
		payload = append(payload, promptdomain.PromptKeywordItem{
			KeywordID: item.KeywordID,
			Word:      strings.TrimSpace(item.Word),
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
		weight := clampWeight(item.Weight)
		words = append(words, fmt.Sprintf("%s（相关度 %d/%d）", word, weight, maxKeywordWeight))
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
