package prompt

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	modeldomain "electron-go-app/backend/internal/infra/model/deepseek"
	"electron-go-app/backend/internal/infra/ratelimit"

	"go.uber.org/zap"
)

const (
	defaultFreeTierAlias       = "deepseek"
	defaultFreeTierModel       = "deepseek-chat"
	defaultFreeTierProvider    = "deepseek"
	defaultFreeTierDisplayName = "DeepSeek-Free"
	defaultFreeTierDailyLimit  = 10
	defaultFreeTierWindow      = 24 * time.Hour
)

const (
	// DefaultFreeTierDailyLimit 默认的免费额度日配额。
	DefaultFreeTierDailyLimit = defaultFreeTierDailyLimit
	// DefaultFreeTierWindow 默认的免费额度计数窗口。
	DefaultFreeTierWindow = defaultFreeTierWindow
)

// ErrFreeTierQuotaExceeded 表示免费额度已被使用完毕。
var ErrFreeTierQuotaExceeded = errors.New("free tier quota exceeded")

// FreeTierQuotaExceededError 描述免费额度耗尽时的具体信息。
type FreeTierQuotaExceededError struct {
	RetryAfter time.Duration
	Remaining  int
}

// Error 返回用户可读的中文提示。
func (e *FreeTierQuotaExceededError) Error() string {
	return "今日免费额度已用尽，请配置模型凭据或等待额度重置。"
}

// Is 允许通过 errors.Is 判断是否为免费额度错误。
func (e *FreeTierQuotaExceededError) Is(target error) bool {
	return target == ErrFreeTierQuotaExceeded
}

// FreeTierConfig 描述免费额度模型的配置项。
type FreeTierConfig struct {
	Enabled     bool          // 是否启用免费额度
	Provider    string        // 模型提供方标识
	Alias       string        // 对外暴露的模型键
	ActualModel string        // 真正调用的模型键
	DisplayName string        // 展示名称
	APIKey      string        // 平台级 API Key
	BaseURL     string        // 自定义 BaseURL（可选）
	DailyQuota  int           // 每日可用次数
	Window      time.Duration // 计数窗口
	Invoker     ModelInvoker  // 自定义调用器（测试用）
}

// normalize 对配置项进行缺省填充与清洗。
func (cfg FreeTierConfig) normalize() FreeTierConfig {
	cfg.Provider = strings.TrimSpace(cfg.Provider)
	if cfg.Provider == "" {
		cfg.Provider = defaultFreeTierProvider
	}
	cfg.Alias = strings.TrimSpace(cfg.Alias)
	if cfg.Alias == "" {
		cfg.Alias = defaultFreeTierAlias
	}
	cfg.ActualModel = strings.TrimSpace(cfg.ActualModel)
	if cfg.ActualModel == "" {
		cfg.ActualModel = defaultFreeTierModel
	}
	cfg.DisplayName = strings.TrimSpace(cfg.DisplayName)
	if cfg.DisplayName == "" {
		cfg.DisplayName = defaultFreeTierDisplayName
	}
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	if cfg.DailyQuota <= 0 {
		cfg.DailyQuota = defaultFreeTierDailyLimit
	}
	if cfg.Window <= 0 {
		cfg.Window = defaultFreeTierWindow
	}
	return cfg
}

// freeTier 使用固定的模型调用器+限流器实现免费额度。
type freeTier struct {
	enabled     bool
	provider    string
	alias       string
	actualModel string
	displayName string
	limit       int
	window      time.Duration
	invoker     ModelInvoker
	limiter     ratelimit.Limiter
	logger      *zap.SugaredLogger
}

// freeTierUsage 记录剩余额度等信息，便于上层展示。
type freeTierUsage struct {
	Limit     int
	Remaining int
}

// FreeTierInfo 描述免费额度模型的静态元数据，便于前端展示。
type FreeTierInfo struct {
	Provider    string
	Alias       string
	ActualModel string
	DisplayName string
	DailyQuota  int
}

// buildFreeTier 根据配置与限流器构造免费额度调度器。
func buildFreeTier(cfg FreeTierConfig, limiter ratelimit.Limiter, logger *zap.SugaredLogger) (*freeTier, error) {
	cfg = cfg.normalize()
	if !cfg.Enabled {
		return nil, nil
	}
	if cfg.Invoker != nil {
		if logger == nil {
			logger = zap.NewNop().Sugar()
		}
		return &freeTier{
			enabled:     true,
			provider:    cfg.Provider,
			alias:       cfg.Alias,
			actualModel: cfg.ActualModel,
			displayName: cfg.DisplayName,
			limit:       cfg.DailyQuota,
			window:      cfg.Window,
			invoker:     cfg.Invoker,
			limiter:     limiter,
			logger:      logger.With("component", "prompt.free_tier"),
		}, nil
	}
	if cfg.APIKey == "" {
		return nil, errors.New("免费额度已启用但缺少 API Key")
	}
	var invoker ModelInvoker
	switch strings.ToLower(cfg.Provider) {
	case "deepseek":
		opts := []modeldomain.Option{}
		if cfg.BaseURL != "" {
			opts = append(opts, modeldomain.WithBaseURL(cfg.BaseURL))
		}
		client := modeldomain.NewClient(cfg.APIKey, opts...)
		invoker = &staticDeepSeekInvoker{
			client:       client,
			defaultModel: cfg.ActualModel,
		}
	default:
		return nil, fmt.Errorf("暂不支持的免费额度模型提供方: %s", cfg.Provider)
	}

	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	return &freeTier{
		enabled:     true,
		provider:    cfg.Provider,
		alias:       cfg.Alias,
		actualModel: cfg.ActualModel,
		displayName: cfg.DisplayName,
		limit:       cfg.DailyQuota,
		window:      cfg.Window,
		invoker:     invoker,
		limiter:     limiter,
		logger:      logger.With("component", "prompt.free_tier"),
	}, nil
}

// matches 判断给定模型标识是否应当使用免费额度。
func (f *freeTier) matches(modelKey string) bool {
	if f == nil || !f.enabled {
		return false
	}
	key := strings.TrimSpace(modelKey)
	if key == "" {
		return true
	}
	return strings.EqualFold(key, f.alias) || strings.EqualFold(key, f.actualModel)
}

// defaultAlias 返回默认的免费模型标识。
func (f *freeTier) defaultAlias() string {
	if f == nil {
		return ""
	}
	return f.alias
}

// info 返回当前免费额度的基础元数据。
func (f *freeTier) info() *FreeTierInfo {
	if f == nil || !f.enabled {
		return nil
	}
	actual := strings.TrimSpace(f.actualModel)
	if actual == "" {
		actual = strings.TrimSpace(f.alias)
	}
	return &FreeTierInfo{
		Provider:    f.provider,
		Alias:       f.alias,
		ActualModel: actual,
		DisplayName: f.displayName,
		DailyQuota:  f.limit,
	}
}

// invoke 调用免费额度模型，同时扣减额度。
func (f *freeTier) invoke(ctx context.Context, userID uint, req modeldomain.ChatCompletionRequest) (modeldomain.ChatCompletionResponse, freeTierUsage, error) {
	if f == nil || !f.enabled {
		return modeldomain.ChatCompletionResponse{}, freeTierUsage{}, errors.New("free tier disabled")
	}
	usage, err := f.consume(ctx, userID)
	if err != nil {
		return modeldomain.ChatCompletionResponse{}, freeTierUsage{}, err
	}
	request := req
	request.Model = strings.TrimSpace(request.Model)
	if request.Model == "" || strings.EqualFold(request.Model, f.alias) {
		request.Model = f.actualModel
	}
	resp, err := f.invoker.InvokeChatCompletion(ctx, userID, request.Model, request)
	if err != nil {
		return modeldomain.ChatCompletionResponse{}, usage, err
	}
	return resp, usage, nil
}

// consume 调用限流器扣减一次额度。
func (f *freeTier) consume(ctx context.Context, userID uint) (freeTierUsage, error) {
	if f.limit <= 0 || f.limiter == nil {
		return freeTierUsage{Limit: -1, Remaining: -1}, nil
	}
	key := fmt.Sprintf("prompt:free:%d", userID)
	result, err := f.limiter.Allow(ctx, key, f.limit, f.window)
	if err != nil {
		return freeTierUsage{}, err
	}
	if !result.Allowed {
		return freeTierUsage{}, &FreeTierQuotaExceededError{
			RetryAfter: result.RetryAfter,
			Remaining:  result.Remaining,
		}
	}
	return freeTierUsage{
		Limit:     f.limit,
		Remaining: result.Remaining,
	}, nil
}

// snapshot 返回当前消耗次数与剩余额度。
func (f *freeTier) snapshot(ctx context.Context, userID uint) (freeTierUsage, time.Duration, error) {
	if f == nil || !f.enabled {
		return freeTierUsage{Limit: -1, Remaining: -1}, 0, nil
	}
	if f.limit <= 0 || f.limiter == nil {
		return freeTierUsage{Limit: -1, Remaining: -1}, 0, nil
	}
	key := fmt.Sprintf("prompt:free:%d", userID)
	switch limiter := f.limiter.(type) {
	case interface {
		// 命中 RedisLimiter，带 ctx 调 Peek
		Peek(context.Context, string) (int, time.Duration, error)
	}:
		count, ttl, err := limiter.Peek(ctx, key)
		if err != nil {
			return freeTierUsage{}, 0, err
		}
		remaining := f.limit - count
		if remaining < 0 {
			remaining = 0
		}
		return freeTierUsage{Limit: f.limit, Remaining: remaining}, ttl, nil
	case interface {
		// 命中 MemoryLimiter，直接 call
		Peek(string) (int, time.Duration)
	}:
		count, ttl := limiter.Peek(key)
		remaining := f.limit - count
		if remaining < 0 {
			remaining = 0
		}
		return freeTierUsage{Limit: f.limit, Remaining: remaining}, ttl, nil
	default:
		// 其它实现（目前没有）就返回 “不支持”
		return freeTierUsage{Limit: f.limit, Remaining: -1}, 0, nil
	}
}

// staticDeepSeekInvoker 使用固定凭据直接调用 DeepSeek。
type staticDeepSeekInvoker struct {
	client       *modeldomain.Client
	defaultModel string
}

// InvokeChatCompletion 通过 DeepSeek 客户端发送对话请求。
func (s *staticDeepSeekInvoker) InvokeChatCompletion(ctx context.Context, _ uint, modelKey string, req modeldomain.ChatCompletionRequest) (modeldomain.ChatCompletionResponse, error) {
	if s == nil || s.client == nil {
		return modeldomain.ChatCompletionResponse{}, errors.New("免费额度模型客户端未初始化")
	}
	request := req
	request.Model = strings.TrimSpace(request.Model)
	if request.Model == "" {
		request.Model = strings.TrimSpace(modelKey)
	}
	if request.Model == "" {
		request.Model = strings.TrimSpace(s.defaultModel)
	}
	if request.Model == "" {
		return modeldomain.ChatCompletionResponse{}, errors.New("免费额度模型缺少标识")
	}
	return s.client.ChatCompletion(ctx, request)
}
