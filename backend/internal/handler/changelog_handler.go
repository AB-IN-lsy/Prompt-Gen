/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-12 11:24:03
 * @FilePath: \electron-go-app\backend\internal\handler\changelog_handler.go
 * @LastEditTime: 2025-10-12 11:24:03
 */
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	response "electron-go-app/backend/internal/infra/common"
	appLogger "electron-go-app/backend/internal/infra/logger"
	"electron-go-app/backend/internal/infra/model/deepseek"
	changelogsvc "electron-go-app/backend/internal/service/changelog"
	modelsvc "electron-go-app/backend/internal/service/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ChangelogHandler 提供更新日志的 HTTP 入口。
type ChangelogHandler struct {
	service      *changelogsvc.Service
	modelService *modelsvc.Service
	logger       *zap.SugaredLogger
}

// NewChangelogHandler 构造 handler。
func NewChangelogHandler(service *changelogsvc.Service, modelService *modelsvc.Service) *ChangelogHandler {
	baseLogger := appLogger.S().With("component", "changelog.handler")
	return &ChangelogHandler{service: service, modelService: modelService, logger: baseLogger}
}

type changelogRequest struct {
	Locale      string   `json:"locale" binding:"required"`
	Badge       string   `json:"badge" binding:"required"`
	Title       string   `json:"title" binding:"required"`
	Summary     string   `json:"summary" binding:"required"`
	Items       []string `json:"items" binding:"required,dive,required"`
	PublishedAt string   `json:"published_at" binding:"required"`
	TranslateTo []string `json:"translate_to"`
	ModelKey    string   `json:"translation_model_key"`
}

// List 返回最新的更新日志。
func (h *ChangelogHandler) List(c *gin.Context) {
	locale := c.DefaultQuery("locale", "en")

	entries, err := h.service.ListEntries(c.Request.Context(), locale)
	if err != nil {
		h.logger.Errorw("list changelog failed", "error", err, "locale", locale)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "list changelog failed", nil)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"items": entries}, nil)
}

// Create 新增日志，仅限管理员。
func (h *ChangelogHandler) Create(c *gin.Context) {
	if !isAdmin(c) {
		response.Fail(c, http.StatusForbidden, response.ErrForbidden, "admin privilege required", nil)
		return
	}
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}

	var req changelogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	// 根据请求过滤并标准化翻译目标语言（排除与原文相同的语言、去重）。
	translateTargets := normalizeTranslateTargets(req.Locale, req.TranslateTo)
	if len(translateTargets) > 0 {
		if h.modelService == nil {
			h.logger.Warnw("model service missing when translate requested")
			response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "translation service unavailable", nil)
			return
		}
		if strings.TrimSpace(req.ModelKey) == "" {
			response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "translation_model_key is required when translate_to is provided", nil)
			return
		}
	}

	params := changelogsvc.CreateEntryParams{
		Locale:      req.Locale,
		Badge:       req.Badge,
		Title:       req.Title,
		Summary:     req.Summary,
		Items:       req.Items,
		PublishedAt: req.PublishedAt,
		AuthorID:    &userID,
	}

	entry, err := h.service.CreateEntry(c.Request.Context(), params)
	if err != nil {
		h.logger.Errorw("create changelog failed", "error", err, "locale", req.Locale, "title", req.Title)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	var translatedEntries []changelogsvc.Entry
	if len(translateTargets) > 0 {
		results, translateErr := h.translateAndCreate(c.Request.Context(), userID, params, translateTargets, strings.TrimSpace(req.ModelKey))
		if translateErr != nil {
			h.logger.Errorw("translate changelog failed", "error", translateErr, "locale", req.Locale, "targets", translateTargets)
			response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, translateErr.Error(), nil)
			return
		}
		translatedEntries = results
	}

	response.Created(c, gin.H{"entry": entry, "translations": translatedEntries}, nil)
}

// Update 编辑日志，仅限管理员。
func (h *ChangelogHandler) Update(c *gin.Context) {
	if !isAdmin(c) {
		response.Fail(c, http.StatusForbidden, response.ErrForbidden, "admin privilege required", nil)
		return
	}

	idParam := c.Param("id")
	id64, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil || id64 == 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "invalid id", nil)
		return
	}

	var req changelogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	entry, err := h.service.UpdateEntry(c.Request.Context(), uint(id64), changelogsvc.UpdateEntryParams{
		Locale:      req.Locale,
		Badge:       req.Badge,
		Title:       req.Title,
		Summary:     req.Summary,
		Items:       req.Items,
		PublishedAt: req.PublishedAt,
	})
	if err != nil {
		if err == changelogsvc.ErrEntryNotFound {
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, err.Error(), nil)
			return
		}
		h.logger.Errorw("update changelog failed", "error", err, "id", id64)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"entry": entry}, nil)
}

// Delete 删除日志，仅限管理员。
func (h *ChangelogHandler) Delete(c *gin.Context) {
	if !isAdmin(c) {
		response.Fail(c, http.StatusForbidden, response.ErrForbidden, "admin privilege required", nil)
		return
	}

	idParam := c.Param("id")
	id64, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil || id64 == 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "invalid id", nil)
		return
	}

	if err := h.service.DeleteEntry(c.Request.Context(), uint(id64)); err != nil {
		if err == changelogsvc.ErrEntryNotFound {
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, err.Error(), nil)
			return
		}
		h.logger.Errorw("delete changelog failed", "error", err, "id", id64)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, err.Error(), nil)
		return
	}

	response.NoContent(c)
}

// translateAndCreate 会针对每个目标语言调用 LLM 获取翻译，并把结果保存为新的 changelog 记录。
func (h *ChangelogHandler) translateAndCreate(ctx context.Context, userID uint, base changelogsvc.CreateEntryParams, targets []string, modelKey string) ([]changelogsvc.Entry, error) {
	results := make([]changelogsvc.Entry, 0, len(targets))
	for _, target := range targets {
		params, err := h.requestTranslation(ctx, userID, base, target, modelKey)
		if err != nil {
			return nil, err
		}
		entry, err := h.service.CreateEntry(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("create translated entry (%s): %w", target, err)
		}
		results = append(results, entry)
	}
	return results, nil
}

// requestTranslation 将原始 changelog 内容封装成提示词，调用 DeepSeek 完成翻译并解析结果。
func (h *ChangelogHandler) requestTranslation(ctx context.Context, userID uint, base changelogsvc.CreateEntryParams, targetLocale, modelKey string) (changelogsvc.CreateEntryParams, error) {
	payload := map[string]any{
		"badge":   base.Badge,
		"title":   base.Title,
		"summary": base.Summary,
		"items":   base.Items,
	}
	serialized, err := json.Marshal(payload)
	if err != nil {
		return changelogsvc.CreateEntryParams{}, fmt.Errorf("encode payload: %w", err)
	}

	messages := []deepseek.ChatMessage{
		{
			Role:    "system",
			Content: "You are a professional localization assistant. Translate release notes while preserving marketing tone. Always respond using strict JSON.",
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("Source locale: %s\nTarget locale: %s\n\nOriginal entry (JSON):\n%s\n\nReturn JSON with keys title, summary, badge, items (array of strings). Keep the number of items identical to the original.", localeDisplayName(base.Locale), localeDisplayName(targetLocale), serialized),
		},
	}

	request := deepseek.ChatCompletionRequest{
		Model:    modelKey,
		Messages: messages,
		ResponseFormat: map[string]any{
			"type": "json_object",
		},
	}

	// 基于管理员提供的模型 key 调用 DeepSeek。若未来支持其他模型，可在 Service 内扩展。
	resp, err := h.modelService.InvokeDeepSeekChatCompletion(ctx, userID, modelKey, request)
	if err != nil {
		return changelogsvc.CreateEntryParams{}, fmt.Errorf("translate via model %s: %w", modelKey, err)
	}
	if len(resp.Choices) == 0 {
		return changelogsvc.CreateEntryParams{}, fmt.Errorf("translation response empty")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	var output translationOutput
	if err := json.Unmarshal([]byte(content), &output); err != nil {
		return changelogsvc.CreateEntryParams{}, fmt.Errorf("parse translation: %w", err)
	}

	items := sanitizeItems(output.Items, base.Items)
	badge := strings.TrimSpace(output.Badge)
	if badge == "" {
		badge = base.Badge
	}

	return changelogsvc.CreateEntryParams{
		Locale:      targetLocale,
		Badge:       badge,
		Title:       strings.TrimSpace(output.Title),
		Summary:     strings.TrimSpace(output.Summary),
		Items:       items,
		PublishedAt: base.PublishedAt,
		AuthorID:    base.AuthorID,
	}, nil
}

type translationOutput struct {
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	Badge   string   `json:"badge"`
	Items   []string `json:"items"`
}

func sanitizeItems(items []string, fallback []string) []string {
	clean := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			clean = append(clean, trimmed)
		}
	}
	if len(clean) == 0 {
		return append([]string{}, fallback...)
	}
	if len(clean) != len(fallback) {
		limit := min(len(clean), len(fallback))
		return append([]string{}, clean[:limit]...)
	}
	return clean
}

func normalizeTranslateTargets(base string, targets []string) []string {
	baseLocale := canonicalLocale(base)
	seen := map[string]struct{}{}
	result := make([]string, 0, len(targets))
	for _, target := range targets {
		locale := canonicalLocale(target)
		if locale == "" || locale == baseLocale {
			continue
		}
		if _, exists := seen[locale]; exists {
			continue
		}
		seen[locale] = struct{}{}
		result = append(result, locale)
	}
	return result
}

func canonicalLocale(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	switch lower {
	case "zh", "zh-cn":
		return "zh-CN"
	case "en", "en-us", "en-gb":
		return "en"
	default:
		return trimmed
	}
}

func localeDisplayName(locale string) string {
	switch canonicalLocale(locale) {
	case "zh-CN":
		return "Simplified Chinese"
	case "en":
		return "English"
	default:
		return locale
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
