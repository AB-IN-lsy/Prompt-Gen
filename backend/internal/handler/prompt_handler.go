package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	response "electron-go-app/backend/internal/infra/common"
	appLogger "electron-go-app/backend/internal/infra/logger"
	"electron-go-app/backend/internal/infra/ratelimit"
	promptsvc "electron-go-app/backend/internal/service/prompt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PromptHandler 承接 Prompt 工作台相关的 HTTP 请求，协调限流与业务服务。
type PromptHandler struct {
	service         *promptsvc.Service
	limiter         ratelimit.Limiter
	logger          *zap.SugaredLogger
	interpretLimit  int
	interpretWindow time.Duration
	generateLimit   int
	generateWindow  time.Duration
}

// PromptRateLimit 配置两类关键接口的限流阈值。
type PromptRateLimit struct {
	InterpretLimit  int
	InterpretWindow time.Duration
	GenerateLimit   int
	GenerateWindow  time.Duration
}

const (
	// DefaultInterpretLimit 控制解析接口默认限额（次/窗口）。
	DefaultInterpretLimit = 8
	// DefaultGenerateLimit 控制生成接口默认限额（次/窗口）。
	DefaultGenerateLimit = 5
	// DefaultInterpretWindow 控制解析接口限流窗口长度。
	DefaultInterpretWindow = time.Minute
	// DefaultGenerateWindow 控制生成接口限流窗口长度。
	DefaultGenerateWindow = time.Minute
)

// NewPromptHandler 创建 PromptHandler，若未传入限流配置则使用默认阈值。
func NewPromptHandler(service *promptsvc.Service, limiter ratelimit.Limiter, cfg PromptRateLimit) *PromptHandler {
	base := appLogger.S().With("component", "prompt.handler")
	if cfg.InterpretLimit <= 0 {
		cfg.InterpretLimit = DefaultInterpretLimit
	}
	if cfg.InterpretWindow <= 0 {
		cfg.InterpretWindow = DefaultInterpretWindow
	}
	if cfg.GenerateLimit <= 0 {
		cfg.GenerateLimit = DefaultGenerateLimit
	}
	if cfg.GenerateWindow <= 0 {
		cfg.GenerateWindow = DefaultGenerateWindow
	}
	return &PromptHandler{
		service:         service,
		limiter:         limiter,
		logger:          base,
		interpretLimit:  cfg.InterpretLimit,
		interpretWindow: cfg.InterpretWindow,
		generateLimit:   cfg.GenerateLimit,
		generateWindow:  cfg.GenerateWindow,
	}
}

// interpretRequest 描述自然语言解析接口的入参。
type interpretRequest struct {
	Description string `json:"description" binding:"required"`
	ModelKey    string `json:"model_key" binding:"required"`
	Language    string `json:"language"`
}

// KeywordPayload 复用前端传递的关键词结构。
type KeywordPayload struct {
	KeywordID uint   `json:"keyword_id"`
	Word      string `json:"word" binding:"required"`
	Source    string `json:"source"`
	Polarity  string `json:"polarity"`
}

// augmentRequest 描述补充关键词的请求体。
type augmentRequest struct {
	Topic            string           `json:"topic" binding:"required"`
	ModelKey         string           `json:"model_key" binding:"required"`
	Language         string           `json:"language"`
	PositiveLimit    int              `json:"positive_limit"`
	NegativeLimit    int              `json:"negative_limit"`
	ExistingPositive []KeywordPayload `json:"existing_positive"`
	ExistingNegative []KeywordPayload `json:"existing_negative"`
	WorkspaceToken   string           `json:"workspace_token"`
}

// manualKeywordRequest 负责接收手动新增关键词的参数。
type manualKeywordRequest struct {
	Topic          string `json:"topic" binding:"required"`
	Word           string `json:"word" binding:"required"`
	Polarity       string `json:"polarity"`
	Language       string `json:"language"`
	PromptID       uint   `json:"prompt_id"`
	WorkspaceToken string `json:"workspace_token"`
}

// generateRequest 描述生成 Prompt 的输入。
type generateRequest struct {
	Topic             string           `json:"topic" binding:"required"`
	ModelKey          string           `json:"model_key" binding:"required"`
	Language          string           `json:"language"`
	Instructions      string           `json:"instructions"`
	Tone              string           `json:"tone"`
	Temperature       float64          `json:"temperature"`
	MaxTokens         int              `json:"max_tokens"`
	PromptID          uint             `json:"prompt_id"`
	IncludeKeywordRef bool             `json:"include_keyword_reference"`
	PositiveKeywords  []KeywordPayload `json:"positive_keywords" binding:"required,dive"`
	NegativeKeywords  []KeywordPayload `json:"negative_keywords"`
	WorkspaceToken    string           `json:"workspace_token"`
}

// saveRequest 接收保存草稿或发布 Prompt 的参数。
type saveRequest struct {
	PromptID         uint             `json:"prompt_id"`
	Topic            string           `json:"topic"`
	Body             string           `json:"body"`
	Model            string           `json:"model"`
	Status           string           `json:"status"`
	Publish          bool             `json:"publish"`
	Tags             []string         `json:"tags"`
	PositiveKeywords []KeywordPayload `json:"positive_keywords" binding:"required,dive"`
	NegativeKeywords []KeywordPayload `json:"negative_keywords"`
	WorkspaceToken   string           `json:"workspace_token"`
}

// Interpret 解析自然语言描述，返回主题与关键词建议。
func (h *PromptHandler) Interpret(c *gin.Context) {
	log := h.scope("interpret")

	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}

	if !h.allow(c, fmt.Sprintf("interpret:%d", userID), h.interpretLimit, h.interpretWindow) {
		return
	}

	var req interpretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	result, err := h.service.Interpret(c.Request.Context(), promptsvc.InterpretInput{
		UserID:      userID,
		Description: req.Description,
		ModelKey:    req.ModelKey,
		Language:    req.Language,
	})
	if err != nil {
		log.Errorw("interpret failed", "error", err, "user_id", userID)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"topic":             result.Topic,
		"positive_keywords": toKeywordResponse(result.PositiveKeywords),
		"negative_keywords": toKeywordResponse(result.NegativeKeywords),
		"confidence":        result.Confidence,
		"workspace_token":   result.WorkspaceToken,
	}, nil)
}

// AugmentKeywords 让模型补充更多关键词并自动去重。
func (h *PromptHandler) AugmentKeywords(c *gin.Context) {
	log := h.scope("augment_keywords")

	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}

	var req augmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	out, err := h.service.AugmentKeywords(c.Request.Context(), promptsvc.AugmentInput{
		UserID:            userID,
		Topic:             req.Topic,
		ModelKey:          req.ModelKey,
		Language:          req.Language,
		WorkspaceToken:    strings.TrimSpace(req.WorkspaceToken),
		RequestedPositive: req.PositiveLimit,
		RequestedNegative: req.NegativeLimit,
		ExistingPositive:  toServiceKeywords(req.ExistingPositive),
		ExistingNegative:  toServiceKeywords(req.ExistingNegative),
	})
	if err != nil {
		log.Errorw("augment keywords failed", "error", err, "user_id", userID)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"positive": toKeywordResponse(out.Positive),
		"negative": toKeywordResponse(out.Negative),
	}, nil)
}

// AddManualKeyword 处理手动关键词录入，立即落库供后续复用。
func (h *PromptHandler) AddManualKeyword(c *gin.Context) {
	log := h.scope("manual_keyword")

	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}

	var req manualKeywordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	item, err := h.service.AddManualKeyword(c.Request.Context(), promptsvc.ManualKeywordInput{
		UserID:         userID,
		Topic:          req.Topic,
		Word:           req.Word,
		Polarity:       req.Polarity,
		Language:       req.Language,
		PromptID:       req.PromptID,
		WorkspaceToken: strings.TrimSpace(req.WorkspaceToken),
	})
	if err != nil {
		log.Warnw("add manual keyword failed", "error", err, "user_id", userID)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	response.Success(c, http.StatusCreated, item, nil)
}

// GeneratePrompt 调用大模型生成 Prompt，带限流保护。
func (h *PromptHandler) GeneratePrompt(c *gin.Context) {
	log := h.scope("generate")

	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}

	if !h.allow(c, fmt.Sprintf("generate:%d", userID), h.generateLimit, h.generateWindow) {
		return
	}

	var req generateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	out, err := h.service.GeneratePrompt(c.Request.Context(), promptsvc.GenerateInput{
		UserID:            userID,
		Topic:             req.Topic,
		ModelKey:          req.ModelKey,
		Language:          req.Language,
		Instructions:      req.Instructions,
		Tone:              req.Tone,
		Temperature:       req.Temperature,
		MaxTokens:         req.MaxTokens,
		PromptID:          req.PromptID,
		IncludeKeywordRef: req.IncludeKeywordRef,
		PositiveKeywords:  toServiceKeywords(req.PositiveKeywords),
		NegativeKeywords:  toServiceKeywords(req.NegativeKeywords),
		WorkspaceToken:    strings.TrimSpace(req.WorkspaceToken),
	})
	if err != nil {
		log.Errorw("generate prompt failed", "error", err, "user_id", userID)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	payload := gin.H{
		"prompt":            out.Prompt,
		"model":             out.Model,
		"duration_ms":       out.Duration.Milliseconds(),
		"positive_keywords": toKeywordResponse(out.PositiveUsed),
		"negative_keywords": toKeywordResponse(out.NegativeUsed),
	}
	if out.Usage != nil {
		payload["usage"] = out.Usage
	}
	if token := strings.TrimSpace(req.WorkspaceToken); token != "" {
		payload["workspace_token"] = token
	}
	response.Success(c, http.StatusOK, payload, nil)
}

// SavePrompt 保存或发布 Prompt，并同步版本号。
func (h *PromptHandler) SavePrompt(c *gin.Context) {
	log := h.scope("save")

	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}

	var req saveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	result, err := h.service.Save(c.Request.Context(), promptsvc.SaveInput{
		UserID:           userID,
		PromptID:         req.PromptID,
		Topic:            req.Topic,
		Body:             req.Body,
		Model:            req.Model,
		Status:           req.Status,
		Publish:          req.Publish,
		Tags:             req.Tags,
		PositiveKeywords: toServiceKeywords(req.PositiveKeywords),
		NegativeKeywords: toServiceKeywords(req.NegativeKeywords),
		WorkspaceToken:   strings.TrimSpace(req.WorkspaceToken),
	})
	if err != nil {
		log.Errorw("save prompt failed", "error", err, "user_id", userID, "prompt_id", req.PromptID)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	response.Success(c, http.StatusOK, result, nil)
}

func (h *PromptHandler) allow(c *gin.Context, key string, limit int, window time.Duration) bool {
	if h.limiter == nil || limit <= 0 {
		return true
	}
	res, err := h.limiter.Allow(c.Request.Context(), key, limit, window)
	if err != nil {
		h.logger.Warnw("ratelimit error", "key", key, "error", err)
		return true
	}
	if res.Allowed {
		return true
	}
	retry := int(res.RetryAfter.Seconds())
	response.Fail(c, http.StatusTooManyRequests, response.ErrTooManyRequests, "请求过于频繁，请稍后再试", gin.H{"retry_after_seconds": retry})
	return false
}

func (h *PromptHandler) scope(action string) *zap.SugaredLogger {
	return h.logger.With("action", action)
}

func toServiceKeywords(items []KeywordPayload) []promptsvc.KeywordItem {
	result := make([]promptsvc.KeywordItem, 0, len(items))
	for _, item := range items {
		result = append(result, promptsvc.KeywordItem{
			KeywordID: item.KeywordID,
			Word:      item.Word,
			Source:    item.Source,
			Polarity:  item.Polarity,
		})
	}
	return result
}

func toKeywordResponse(items []promptsvc.KeywordItem) []gin.H {
	result := make([]gin.H, 0, len(items))
	for _, item := range items {
		result = append(result, gin.H{
			"keyword_id": item.KeywordID,
			"word":       item.Word,
			"source":     item.Source,
			"polarity":   item.Polarity,
		})
	}
	return result
}
