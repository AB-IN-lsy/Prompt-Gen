/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-11 00:50:55
 * @FilePath: \electron-go-app\backend\internal\handler\model_handler.go
 * @LastEditTime: 2025-10-11 00:51:01
 */
package handler

import (
	"fmt"
	"net/http"
	"strconv"

	response "electron-go-app/backend/internal/infra/common"
	appLogger "electron-go-app/backend/internal/infra/logger"
	modelsvc "electron-go-app/backend/internal/service/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ModelHandler 负责模型凭据相关接口。
type ModelHandler struct {
	service *modelsvc.Service
	logger  *zap.SugaredLogger
}

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
			return
		default:
			log.Errorw("update credential failed", "error", err, "user_id", userID, "credential_id", id)
			response.Fail(c, http.StatusInternalServerError, response.ErrInternal, err.Error(), nil)
			return
		}
	}
	response.Success(c, http.StatusOK, cred, nil)
}

// Delete 移除模型凭据。
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

func (h *ModelHandler) scope(operation string) *zap.SugaredLogger {
	if h.logger == nil {
		h.logger = appLogger.S().With("component", "model.handler")
	}
	return h.logger.With("operation", operation)
}

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
