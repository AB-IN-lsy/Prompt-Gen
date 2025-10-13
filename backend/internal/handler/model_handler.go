/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-11 00:50:55
 * @FilePath: \electron-go-app\backend\internal\handler\model_handler.go
 * @LastEditTime: 2025-10-11 00:51:01
 */
package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	response "electron-go-app/backend/internal/infra/common"
	appLogger "electron-go-app/backend/internal/infra/logger"
	deepseek "electron-go-app/backend/internal/infra/model/deepseek"
	modelsvc "electron-go-app/backend/internal/service/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ModelHandler 负责模型凭据相关接口。
type ModelHandler struct {
	service *modelsvc.Service
	logger  *zap.SugaredLogger
}

// NewModelHandler 构造模型凭据相关的 Handler。
func NewModelHandler(service *modelsvc.Service) *ModelHandler {
	base := appLogger.S().With("component", "model.handler")
	return &ModelHandler{service: service, logger: base}
}

// List 返回当前用户的模型凭据列表。
func (h *ModelHandler) List(c *gin.Context) {
	log := h.scope("list")
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}
	creds, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		log.Errorw("list credentials failed", "error", err, "user_id", userID)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, err.Error(), nil)
		return
	}
	response.Success(c, http.StatusOK, creds, nil)
}

// CreateRequest 表示创建模型凭据的请求体。
type CreateRequest struct {
	Provider    string                 `json:"provider" binding:"required"`
	ModelKey    string                 `json:"model_key" binding:"required"`
	DisplayName string                 `json:"display_name" binding:"required"`
	BaseURL     string                 `json:"base_url"`
	APIKey      string                 `json:"api_key" binding:"required"`
	ExtraConfig map[string]interface{} `json:"extra_config"`
}

// Create 新增模型凭据。
func (h *ModelHandler) Create(c *gin.Context) {
	log := h.scope("create")
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}
	cred, err := h.service.Create(c.Request.Context(), userID, modelsvc.CreateInput{
		Provider:    req.Provider,
		ModelKey:    req.ModelKey,
		DisplayName: req.DisplayName,
		BaseURL:     req.BaseURL,
		APIKey:      req.APIKey,
		ExtraConfig: req.ExtraConfig,
	})
	if err != nil {
		switch err {
		case modelsvc.ErrDuplicatedModelKey:
			response.Fail(c, http.StatusConflict, response.ErrConflict, err.Error(), gin.H{"field": "model_key"})
			return
		case modelsvc.ErrUnsupportedProvider:
			response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), gin.H{"field": "provider"})
			return
		default:
			log.Errorw("create credential failed", "error", err, "user_id", userID)
			response.Fail(c, http.StatusInternalServerError, response.ErrInternal, err.Error(), nil)
			return
		}
	}
	response.Success(c, http.StatusCreated, cred, nil)
}

// UpdateRequest 表示更新模型凭据的请求体。
type UpdateRequest struct {
	DisplayName *string                `json:"display_name"`
	BaseURL     *string                `json:"base_url"`
	APIKey      *string                `json:"api_key"`
	ExtraConfig map[string]interface{} `json:"extra_config"`
	Status      *string                `json:"status"`
}

// TestConnectionRequest 允许前端自定义测试 prompt 或消息体。
type TestConnectionRequest struct {
	Model    string                 `json:"model"`
	Prompt   string                 `json:"prompt"`
	Messages []deepseek.ChatMessage `json:"messages"`
}

// Update 修改指定凭据。
func (h *ModelHandler) Update(c *gin.Context) {
	log := h.scope("update")
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}
	cred, err := h.service.Update(c.Request.Context(), userID, id, modelsvc.UpdateInput{
		DisplayName: req.DisplayName,
		BaseURL:     req.BaseURL,
		APIKey:      req.APIKey,
		ExtraConfig: req.ExtraConfig,
		Status:      req.Status,
	})
	if err != nil {
		switch err {
		case modelsvc.ErrCredentialNotFound:
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, err.Error(), nil)
			return
		case modelsvc.ErrInvalidStatus:
			response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), gin.H{"field": "status"})
		case modelsvc.ErrUnsupportedProvider:
			response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), gin.H{"field": "provider"})
			return
		default:
			log.Errorw("update credential failed", "error", err, "user_id", userID, "credential_id", id)
			response.Fail(c, http.StatusInternalServerError, response.ErrInternal, err.Error(), nil)
			return
		}
	}
	response.Success(c, http.StatusOK, cred, nil)
}

// TestConnection 尝试使用凭据请求大模型，返回第一手响应。
func (h *ModelHandler) TestConnection(c *gin.Context) {
	log := h.scope("test_connection")
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	var req TestConnectionRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil && !errors.Is(bindErr, io.EOF) {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, bindErr.Error(), nil)
		return
	}

	chatReq := deepseek.ChatCompletionRequest{
		Model: strings.TrimSpace(req.Model),
	}
	if len(req.Messages) > 0 {
		chatReq.Messages = req.Messages
	}

	prompt := strings.TrimSpace(req.Prompt)
	if len(chatReq.Messages) == 0 {
		if prompt == "" {
			prompt = "请回复“pong”以确认连通性。"
		}
		// 默认拼出最简对话，让测试在没有自定义消息时也能正常执行。
		chatReq.Messages = []deepseek.ChatMessage{
			{Role: "system", Content: "你正在协助验证 API 凭据是否可用，请简短确认。"},
			{Role: "user", Content: prompt},
		}
	}

	resp, err := h.service.TestConnection(c.Request.Context(), userID, id, chatReq)
	if err != nil {
		switch {
		case errors.Is(err, modelsvc.ErrCredentialNotFound):
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, err.Error(), nil)
		case errors.Is(err, modelsvc.ErrCredentialDisabled):
			response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		default:
			if apiErr, ok := err.(*deepseek.APIError); ok {
				response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, apiErr.Error(), gin.H{
					"status_code": apiErr.StatusCode,
					"type":        apiErr.Type,
					"code":        apiErr.Code,
				})
				return
			}
			log.Errorw("test credential failed", "error", err, "user_id", userID, "credential_id", id)
			response.Fail(c, http.StatusInternalServerError, response.ErrInternal, err.Error(), nil)
		}
		return
	}

	response.Success(c, http.StatusOK, resp, nil)
}

// Delete 移除指定的模型凭据。
func (h *ModelHandler) Delete(c *gin.Context) {
	log := h.scope("delete")
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}
	if err := h.service.Delete(c.Request.Context(), userID, id); err != nil {
		switch err {
		case modelsvc.ErrCredentialNotFound:
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, err.Error(), nil)
			return
		default:
			log.Errorw("delete credential failed", "error", err, "user_id", userID, "credential_id", id)
			response.Fail(c, http.StatusInternalServerError, response.ErrInternal, err.Error(), nil)
			return
		}
	}
	response.NoContent(c)
}

// scope 派生带操作标签的日志实例，方便排查请求行为。
func (h *ModelHandler) scope(operation string) *zap.SugaredLogger {
	if h.logger == nil {
		h.logger = appLogger.S().With("component", "model.handler")
	}
	return h.logger.With("operation", operation)
}

// parseUintParam 从路径参数解析无符号整数。
func parseUintParam(c *gin.Context, name string) (uint, error) {
	val := c.Param(name)
	if val == "" {
		return 0, fmt.Errorf("missing path parameter: %s", name)
	}
	parsed, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return uint(parsed), nil
}
