package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	promptdomain "electron-go-app/backend/internal/domain/prompt"
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
	saveLimit       int
	saveWindow      time.Duration
	publishLimit    int
	publishWindow   time.Duration
}

// PromptRateLimit 配置两类关键接口的限流阈值。
type PromptRateLimit struct {
	InterpretLimit  int
	InterpretWindow time.Duration
	GenerateLimit   int
	GenerateWindow  time.Duration
	SaveLimit       int
	SaveWindow      time.Duration
	PublishLimit    int
	PublishWindow   time.Duration
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
	// DefaultSaveLimit 控制保存接口默认限额。
	DefaultSaveLimit = 20
	// DefaultSaveWindow 控制保存接口限流窗口长度。
	DefaultSaveWindow = time.Minute
	// DefaultPublishLimit 控制发布接口默认限额。
	DefaultPublishLimit = 6
	// DefaultPublishWindow 控制发布接口限流窗口长度。
	DefaultPublishWindow = 10 * time.Minute
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
	if cfg.SaveLimit <= 0 {
		cfg.SaveLimit = DefaultSaveLimit
	}
	if cfg.SaveWindow <= 0 {
		cfg.SaveWindow = DefaultSaveWindow
	}
	if cfg.PublishLimit <= 0 {
		cfg.PublishLimit = DefaultPublishLimit
	}
	if cfg.PublishWindow <= 0 {
		cfg.PublishWindow = DefaultPublishWindow
	}
	return &PromptHandler{
		service:         service,
		limiter:         limiter,
		logger:          base,
		interpretLimit:  cfg.InterpretLimit,
		interpretWindow: cfg.InterpretWindow,
		generateLimit:   cfg.GenerateLimit,
		generateWindow:  cfg.GenerateWindow,
		saveLimit:       cfg.SaveLimit,
		saveWindow:      cfg.SaveWindow,
		publishLimit:    cfg.PublishLimit,
		publishWindow:   cfg.PublishWindow,
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
	Weight    int    `json:"weight"`
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
	Weight         int    `json:"weight"`
}

// removeKeywordRequest 用于同步移除工作区中的关键词。
type removeKeywordRequest struct {
	Word           string `json:"word" binding:"required"`
	Polarity       string `json:"polarity"`
	WorkspaceToken string `json:"workspace_token"`
}

type syncWorkspaceRequest struct {
	WorkspaceToken   string           `json:"workspace_token" binding:"required"`
	PositiveKeywords []KeywordPayload `json:"positive_keywords"`
	NegativeKeywords []KeywordPayload `json:"negative_keywords"`
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
	Instructions     string           `json:"instructions"`
	Model            string           `json:"model"`
	Status           string           `json:"status"`
	Publish          bool             `json:"publish"`
	Tags             []string         `json:"tags"`
	PositiveKeywords []KeywordPayload `json:"positive_keywords" binding:"required,dive"`
	NegativeKeywords []KeywordPayload `json:"negative_keywords"`
	WorkspaceToken   string           `json:"workspace_token"`
}

// ListPrompts 返回当前登录用户的 Prompt 列表。
func (h *PromptHandler) ListPrompts(c *gin.Context) {
	log := h.scope("list")
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}

	defaultPageSize, maxPageSize := h.service.ListPageSizeDefaults()
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page <= 0 {
		page = 1
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil || pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	out, err := h.service.ListPrompts(c.Request.Context(), promptsvc.ListPromptsInput{
		UserID:   userID,
		Status:   c.Query("status"),
		Query:    c.Query("q"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		log.Errorw("list prompts failed", "error", err, "user_id", userID)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "获取 Prompt 列表失败", nil)
		return
	}

	items := make([]gin.H, 0, len(out.Items))
	for _, item := range out.Items {
		items = append(items, gin.H{
			"id":                item.ID,
			"topic":             item.Topic,
			"model":             item.Model,
			"status":            item.Status,
			"tags":              item.Tags,
			"positive_keywords": toKeywordResponse(item.PositiveKeywords),
			"negative_keywords": toKeywordResponse(item.NegativeKeywords),
			"updated_at":        item.UpdatedAt,
			"published_at":      item.PublishedAt,
		})
	}

	totalPages := 0
	if out.PageSize > 0 {
		totalPages = int((out.Total + int64(out.PageSize) - 1) / int64(out.PageSize))
	}

	response.Success(
		c,
		http.StatusOK,
		gin.H{"items": items},
		response.MetaPagination{
			Page:       out.Page,
			PageSize:   out.PageSize,
			TotalItems: int(out.Total),
			TotalPages: totalPages,
		},
	)
}

// ExportPrompts 将当前用户的 Prompt 导出为本地文件并返回保存路径。
func (h *PromptHandler) ExportPrompts(c *gin.Context) {
	log := h.scope("export")
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}

	result, err := h.service.ExportPrompts(c.Request.Context(), promptsvc.ExportPromptsInput{
		UserID: userID,
	})
	if err != nil {
		log.Errorw("export prompts failed", "error", err, "user_id", userID)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "导出 Prompt 失败", nil)
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"file_path":    result.FilePath,
		"prompt_count": result.PromptCount,
		"generated_at": result.GeneratedAt,
	}, nil)
}

// ImportPrompts 读取导出文件内容并批量导入 Prompt 数据。
func (h *PromptHandler) ImportPrompts(c *gin.Context) {
	log := h.scope("import")
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}

	mode := strings.TrimSpace(c.Query("mode"))
	if mode == "" {
		mode = strings.TrimSpace(c.PostForm("mode"))
	}

	payload, err := h.readImportPayload(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "导入数据读取失败", gin.H{"detail": err.Error()})
		return
	}

	result, err := h.service.ImportPrompts(c.Request.Context(), promptsvc.ImportPromptsInput{
		UserID:  userID,
		Mode:    mode,
		Payload: payload,
	})
	if err != nil {
		log.Errorw("import prompts failed", "error", err, "user_id", userID)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "导入 Prompt 失败", nil)
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"imported_count": result.Imported,
		"skipped_count":  result.Skipped,
		"errors":         result.Errors,
	}, nil)
}

// ListPromptVersions 返回指定 Prompt 的历史版本列表。
func (h *PromptHandler) ListPromptVersions(c *gin.Context) {
	log := h.scope("list_versions")
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}
	promptID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || promptID == 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "invalid prompt id", nil)
		return
	}
	limit := 0
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		if parsed, parseErr := strconv.Atoi(rawLimit); parseErr == nil && parsed > 0 {
			limit = parsed
		}
	}
	result, err := h.service.ListPromptVersions(c.Request.Context(), promptsvc.ListVersionsInput{
		UserID:   userID,
		PromptID: uint(promptID),
		Limit:    limit,
	})
	if err != nil {
		if errors.Is(err, promptsvc.ErrPromptNotFound) {
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, "prompt not found", nil)
			return
		}
		log.Errorw("list prompt versions failed", "error", err, "user_id", userID, "prompt_id", promptID)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "获取历史版本失败", nil)
		return
	}
	items := make([]gin.H, 0, len(result.Versions))
	for _, version := range result.Versions {
		items = append(items, gin.H{
			"version_no": version.VersionNo,
			"model":      version.Model,
			"created_at": version.CreatedAt,
		})
	}
	response.Success(c, http.StatusOK, gin.H{"versions": items}, nil)
}

// GetPromptVersion 返回指定历史版本的详细内容。
func (h *PromptHandler) GetPromptVersion(c *gin.Context) {
	log := h.scope("get_version")
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}
	promptID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || promptID == 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "invalid prompt id", nil)
		return
	}
	versionNo, err := strconv.Atoi(strings.TrimSpace(c.Param("version")))
	if err != nil || versionNo <= 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "invalid version", nil)
		return
	}
	detail, err := h.service.GetPromptVersionDetail(c.Request.Context(), promptsvc.GetVersionDetailInput{
		UserID:    userID,
		PromptID:  uint(promptID),
		VersionNo: versionNo,
	})
	if err != nil {
		if errors.Is(err, promptsvc.ErrPromptNotFound) {
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, "prompt not found", nil)
			return
		}
		if errors.Is(err, promptsvc.ErrPromptVersionNotFound) {
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, "prompt version not found", nil)
			return
		}
		log.Errorw("get prompt version failed", "error", err, "user_id", userID, "prompt_id", promptID, "version", versionNo)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "获取历史版本详情失败", nil)
		return
	}
	response.Success(c, http.StatusOK, gin.H{
		"version_no":        detail.VersionNo,
		"model":             detail.Model,
		"body":              detail.Body,
		"instructions":      detail.Instructions,
		"positive_keywords": toKeywordResponse(detail.PositiveKeywords),
		"negative_keywords": toKeywordResponse(detail.NegativeKeywords),
		"created_at":        detail.CreatedAt,
	}, nil)
}

// GetPrompt 返回指定 Prompt 的详情，并附带最新的工作区 token。
func (h *PromptHandler) GetPrompt(c *gin.Context) {
	log := h.scope("detail")
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}
	promptID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || promptID == 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "invalid prompt id", nil)
		return
	}

	detail, err := h.service.GetPrompt(c.Request.Context(), promptsvc.GetPromptInput{
		UserID:   userID,
		PromptID: uint(promptID),
	})
	if err != nil {
		if errors.Is(err, promptsvc.ErrPromptNotFound) {
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, "prompt not found", nil)
			return
		}
		log.Errorw("get prompt failed", "error", err, "user_id", userID, "prompt_id", promptID)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "获取 Prompt 详情失败", nil)
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"id":                detail.ID,
		"topic":             detail.Topic,
		"body":              detail.Body,
		"instructions":      detail.Instructions,
		"model":             detail.Model,
		"status":            detail.Status,
		"tags":              detail.Tags,
		"positive_keywords": toKeywordResponse(detail.PositiveKeywords),
		"negative_keywords": toKeywordResponse(detail.NegativeKeywords),
		"workspace_token":   detail.WorkspaceToken,
		"created_at":        detail.CreatedAt,
		"updated_at":        detail.UpdatedAt,
		"published_at":      detail.PublishedAt,
	}, nil)
}

// DeletePrompt 删除指定 Prompt。
func (h *PromptHandler) DeletePrompt(c *gin.Context) {
	log := h.scope("delete")
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}
	promptID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || promptID == 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "invalid prompt id", nil)
		return
	}

	if err := h.service.DeletePrompt(c.Request.Context(), promptsvc.DeletePromptInput{
		UserID:   userID,
		PromptID: uint(promptID),
	}); err != nil {
		if errors.Is(err, promptsvc.ErrPromptNotFound) {
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, "prompt not found", nil)
			return
		}
		log.Errorw("delete prompt failed", "error", err, "user_id", userID, "prompt_id", promptID)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "删除 Prompt 失败", nil)
		return
	}

	response.NoContent(c)
}

// Interpret 解析自然语言描述，返回主题与关键词，建议
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
		if errors.Is(err, promptsvc.ErrModelInvocationFailed) {
			response.Fail(c, http.StatusServiceUnavailable, response.ErrInternal, "调用模型失败，请检查网络连接或模型凭据。", nil)
			return
		}
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"topic":             result.Topic,
		"positive_keywords": toKeywordResponse(result.PositiveKeywords),
		"negative_keywords": toKeywordResponse(result.NegativeKeywords),
		"confidence":        result.Confidence,
		"workspace_token":   result.WorkspaceToken,
		"instructions":      result.Instructions,
	}, nil)
}

// AugmentKeywords 让模型补充更多关键词并自动去重，返回新增的关键词列表。
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

	limit := h.service.KeywordLimit()
	if len(req.ExistingPositive) > limit {
		h.keywordLimitError(c, promptdomain.KeywordPolarityPositive, len(req.ExistingPositive))
		return
	}
	if len(req.ExistingNegative) > limit {
		h.keywordLimitError(c, promptdomain.KeywordPolarityNegative, len(req.ExistingNegative))
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
		if errors.Is(err, promptsvc.ErrModelInvocationFailed) {
			response.Fail(c, http.StatusServiceUnavailable, response.ErrInternal, "调用模型失败，请检查网络连接或模型凭据。", nil)
			return
		}
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
		Weight:         req.Weight,
	})
	if err != nil {
		if errors.Is(err, promptsvc.ErrPositiveKeywordLimit) {
			h.keywordLimitError(c, promptdomain.KeywordPolarityPositive, -1)
			return
		}
		if errors.Is(err, promptsvc.ErrNegativeKeywordLimit) {
			h.keywordLimitError(c, promptdomain.KeywordPolarityNegative, -1)
			return
		}
		if errors.Is(err, promptsvc.ErrDuplicateKeyword) {
			h.keywordDuplicateError(c, req.Polarity, req.Word)
			return
		}
		log.Warnw("add manual keyword failed", "error", err, "user_id", userID)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	response.Success(c, http.StatusCreated, item, nil)
}

// RemoveKeyword 同步删除临时工作区的关键词，保持后端缓存与前端一致。
func (h *PromptHandler) RemoveKeyword(c *gin.Context) {
	log := h.scope("remove_keyword")

	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}

	var req removeKeywordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	if err := h.service.RemoveWorkspaceKeyword(c.Request.Context(), promptsvc.RemoveKeywordInput{
		UserID:         userID,
		Word:           req.Word,
		Polarity:       req.Polarity,
		WorkspaceToken: strings.TrimSpace(req.WorkspaceToken),
	}); err != nil {
		log.Warnw("remove keyword failed", "error", err, "user_id", userID)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	response.NoContent(c)
}

// SyncKeywords 将关键词的排序与权重同步到 Redis 工作区。
func (h *PromptHandler) SyncKeywords(c *gin.Context) {
	log := h.scope("sync_keywords")

	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}

	var req syncWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	if err := h.service.SyncWorkspaceKeywords(c.Request.Context(), promptsvc.SyncWorkspaceInput{
		UserID:         userID,
		WorkspaceToken: strings.TrimSpace(req.WorkspaceToken),
		Positive:       toServiceKeywords(req.PositiveKeywords),
		Negative:       toServiceKeywords(req.NegativeKeywords),
	}); err != nil {
		log.Warnw("sync workspace keywords failed", "error", err, "user_id", userID)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	response.NoContent(c)
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

	if !h.validateKeywordLimit(c, req.PositiveKeywords, req.NegativeKeywords) {
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
		if errors.Is(err, promptsvc.ErrPositiveKeywordLimit) {
			h.keywordLimitError(c, promptdomain.KeywordPolarityPositive, len(req.PositiveKeywords))
			return
		}
		if errors.Is(err, promptsvc.ErrNegativeKeywordLimit) {
			h.keywordLimitError(c, promptdomain.KeywordPolarityNegative, len(req.NegativeKeywords))
			return
		}
		log.Errorw("generate prompt failed", "error", err, "user_id", userID)
		if errors.Is(err, promptsvc.ErrModelInvocationFailed) {
			response.Fail(c, http.StatusServiceUnavailable, response.ErrInternal, "调用模型失败，请检查网络连接或模型凭据。", nil)
			return
		}
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	payload := gin.H{
		"prompt":            out.Prompt,
		"model":             out.Model,
		"duration_ms":       out.Duration.Milliseconds(),
		"positive_keywords": toKeywordResponse(out.PositiveUsed),
		"negative_keywords": toKeywordResponse(out.NegativeUsed),
		"topic":             strings.TrimSpace(req.Topic),
	}
	if out.Usage != nil {
		payload["usage"] = out.Usage
	}
	if token := strings.TrimSpace(req.WorkspaceToken); token != "" {
		payload["workspace_token"] = token
	}
	response.Success(c, http.StatusOK, payload, nil)
}

// SavePrompt 保存或发布 Prompt 草稿，并同步工作区元数据。
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

	if !h.allow(c, fmt.Sprintf("prompt:save:%d", userID), h.saveLimit, h.saveWindow) {
		return
	}
	if req.Publish {
		if !h.allow(c, fmt.Sprintf("prompt:publish:%d", userID), h.publishLimit, h.publishWindow) {
			return
		}
	}

	if !h.validateKeywordLimit(c, req.PositiveKeywords, req.NegativeKeywords) {
		return
	}

	result, err := h.service.Save(c.Request.Context(), promptsvc.SaveInput{
		UserID:                   userID,
		PromptID:                 req.PromptID,
		Topic:                    req.Topic,
		Body:                     req.Body,
		Instructions:             req.Instructions,
		Model:                    req.Model,
		Status:                   req.Status,
		Publish:                  req.Publish,
		Tags:                     req.Tags,
		PositiveKeywords:         toServiceKeywords(req.PositiveKeywords),
		NegativeKeywords:         toServiceKeywords(req.NegativeKeywords),
		WorkspaceToken:           strings.TrimSpace(req.WorkspaceToken),
		EnforcePublishValidation: true,
	})
	if err != nil {
		if errors.Is(err, promptsvc.ErrPositiveKeywordLimit) {
			h.keywordLimitError(c, promptdomain.KeywordPolarityPositive, len(req.PositiveKeywords))
			return
		}
		if errors.Is(err, promptsvc.ErrNegativeKeywordLimit) {
			h.keywordLimitError(c, promptdomain.KeywordPolarityNegative, len(req.NegativeKeywords))
			return
		}
		if errors.Is(err, promptsvc.ErrTagLimitExceeded) {
			h.tagLimitError(c, len(req.Tags))
			return
		}
		log.Errorw("save prompt failed", "error", err, "user_id", userID, "prompt_id", req.PromptID)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	response.Success(c, http.StatusOK, result, nil)
}

// allow 根据限流配置判断当前请求是否放行。
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

// readImportPayload 负责从请求中提取导入文件的原始内容。
func (h *PromptHandler) readImportPayload(c *gin.Context) ([]byte, error) {
	contentType := c.ContentType()
	if strings.HasPrefix(contentType, "multipart/form-data") {
		file, err := c.FormFile("file")
		if err != nil {
			return nil, fmt.Errorf("缺少导入文件: %w", err)
		}
		src, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("打开导入文件失败: %w", err)
		}
		defer func() { _ = src.Close() }()
		data, err := io.ReadAll(src)
		if err != nil {
			return nil, fmt.Errorf("读取导入文件失败: %w", err)
		}
		return data, nil
	}
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, fmt.Errorf("读取导入内容失败: %w", err)
	}
	if len(data) == 0 {
		return nil, errors.New("导入内容为空")
	}
	return data, nil
}

// scope 派生带行动标签的日志实例，便于排查具体操作。
func (h *PromptHandler) scope(action string) *zap.SugaredLogger {
	return h.logger.With("action", action)
}

// keywordLimitDetailCode 用于在 details 中标记关键词数量超限的错误，前端可据此显示对应 i18n 文案。
const keywordLimitDetailCode = "KEYWORD_LIMIT"

// keywordDuplicateDetailCode 用于标记关键词重复的错误。
const keywordDuplicateDetailCode = "KEYWORD_DUPLICATE"

// tagLimitDetailCode 用于标记标签数量超限的错误。
const tagLimitDetailCode = "TAG_LIMIT"

// validateKeywordLimit 在进入业务逻辑前做守卫，避免传入超出限制的关键词集合。
func (h *PromptHandler) validateKeywordLimit(c *gin.Context, positive, negative []KeywordPayload) bool {
	limit := h.service.KeywordLimit()
	if len(positive) > limit {
		h.keywordLimitError(c, promptdomain.KeywordPolarityPositive, len(positive))
		return false
	}
	if len(negative) > limit {
		h.keywordLimitError(c, promptdomain.KeywordPolarityNegative, len(negative))
		return false
	}
	return true
}

// keywordLimitError 返回统一的关键词超限错误响应。
func (h *PromptHandler) keywordLimitError(c *gin.Context, polarity string, count int) {
	limit := h.service.KeywordLimit()
	message := fmt.Sprintf("正向关键词最多 %d 个", limit)
	if polarity == promptdomain.KeywordPolarityNegative {
		message = fmt.Sprintf("负向关键词最多 %d 个", limit)
	}
	details := gin.H{
		"code":     keywordLimitDetailCode,
		"polarity": polarity,
		"limit":    limit,
	}
	if count >= 0 {
		details["count"] = count
	}
	response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, message, details)
}

// tagLimitError 返回统一的标签超限错误响应。
func (h *PromptHandler) tagLimitError(c *gin.Context, count int) {
	limit := h.service.TagLimit()
	message := fmt.Sprintf("标签最多 %d 个", limit)
	details := gin.H{
		"code":  tagLimitDetailCode,
		"limit": limit,
	}
	if count >= 0 {
		details["count"] = count
	}
	response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, message, details)
}

// keywordDuplicateError 返回统一的关键词重复错误响应。
func (h *PromptHandler) keywordDuplicateError(c *gin.Context, polarity, word string) {
	pol := promptdomain.KeywordPolarityPositive
	message := "该关键词已存在"
	if strings.ToLower(strings.TrimSpace(polarity)) == promptdomain.KeywordPolarityNegative {
		pol = promptdomain.KeywordPolarityNegative
	}
	details := gin.H{
		"code":     keywordDuplicateDetailCode,
		"polarity": pol,
		"word":     word,
	}
	response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, message, details)
}

func toServiceKeywords(items []KeywordPayload) []promptsvc.KeywordItem {
	result := make([]promptsvc.KeywordItem, 0, len(items))
	for _, item := range items {
		result = append(result, promptsvc.KeywordItem{
			KeywordID: item.KeywordID,
			Word:      item.Word,
			Source:    item.Source,
			Polarity:  item.Polarity,
			Weight:    item.Weight,
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
			"weight":     item.Weight,
		})
	}
	return result
}
