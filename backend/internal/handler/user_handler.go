/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 22:38:26
 * @FilePath: \electron-go-app\backend\internal\handler\user_handler.go
 * @LastEditTime: 2025-10-08 22:38:31
 */
package handler

import (
	"net/http"

	"electron-go-app/backend/internal/handler/response"
	appLogger "electron-go-app/backend/internal/infra/logger"
	usersvc "electron-go-app/backend/internal/service/user"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// UserHandler 负责用户资料相关的 HTTP 入口。
type UserHandler struct {
	service *usersvc.Service
	logger  *zap.SugaredLogger
}

// NewUserHandler 构造用户 handler。
func NewUserHandler(service *usersvc.Service) *UserHandler {
	baseLogger := appLogger.S().With("component", "user.handler")
	return &UserHandler{service: service, logger: baseLogger}
}

// GetMe 返回当前登录用户资料。
func (h *UserHandler) GetMe(c *gin.Context) {
	log := h.scope("get_me")

	userID, ok := extractUserID(c)
	if !ok {
		log.Warnw("missing user id")
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}

	log = log.With("user_id", userID)

	profile, err := h.service.GetProfile(c.Request.Context(), userID)
	if err != nil {
		status := http.StatusInternalServerError
		code := response.ErrInternal
		if err == usersvc.ErrUserNotFound {
			status = http.StatusNotFound
			code = response.ErrNotFound
			log.Warnw("user not found")
		} else {
			log.Errorw("get profile failed", "error", err)
		}
		response.Fail(c, status, code, err.Error(), nil)
		return
	}

	log.Infow("get profile success")

	response.Success(c, http.StatusOK, profile, nil)
}

// UpdateSettingsRequest 描述更新设置的请求体。
type UpdateSettingsRequest struct {
	PreferredModel string `json:"preferred_model" binding:"omitempty,min=1"`
	SyncEnabled    *bool  `json:"sync_enabled"`
}

// UpdateMe 更新当前登录用户的设置。
func (h *UserHandler) UpdateMe(c *gin.Context) {
	log := h.scope("update_me")

	userID, ok := extractUserID(c)
	if !ok {
		log.Warnw("missing user id")
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}

	log = log.With("user_id", userID)

	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnw("invalid request body", "error", err)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	log.Infow("update request", "preferred_model", req.PreferredModel, "sync_enabled", req.SyncEnabled)

	profile, err := h.service.GetProfile(c.Request.Context(), userID)
	if err != nil {
		status := http.StatusInternalServerError
		code := response.ErrInternal
		if err == usersvc.ErrUserNotFound {
			status = http.StatusNotFound
			code = response.ErrNotFound
			log.Warnw("user not found when updating")
		} else {
			log.Errorw("get profile failed", "error", err)
		}
		response.Fail(c, status, code, err.Error(), nil)
		return
	}

	settings := profile.Settings
	if req.PreferredModel != "" {
		settings.PreferredModel = req.PreferredModel
	}
	if req.SyncEnabled != nil {
		settings.SyncEnabled = *req.SyncEnabled
	}

	updated, err := h.service.UpdateSettings(c.Request.Context(), userID, settings)
	if err != nil {
		status := http.StatusInternalServerError
		code := response.ErrInternal
		if err == usersvc.ErrUserNotFound {
			status = http.StatusNotFound
			code = response.ErrNotFound
			log.Warnw("user not found on update")
		} else {
			log.Errorw("update settings failed", "error", err)
		}
		response.Fail(c, status, code, err.Error(), nil)
		return
	}

	log.Infow("update settings success", "preferred_model", updated.Settings.PreferredModel, "sync_enabled", updated.Settings.SyncEnabled)

	response.Success(c, http.StatusOK, updated, nil)
}

func (h *UserHandler) ensureLogger() *zap.SugaredLogger {
	if h.logger == nil {
		h.logger = appLogger.S().With("component", "user.handler")
	}
	return h.logger
}

func (h *UserHandler) scope(operation string) *zap.SugaredLogger {
	return h.ensureLogger().With("operation", operation)
}

func extractUserID(c *gin.Context) (uint, bool) {
	val, ok := c.Get("userID")
	if !ok {
		return 0, false
	}
	switch id := val.(type) {
	case uint:
		return id, true
	case uint64:
		return uint(id), true
	case int:
		if id < 0 {
			return 0, false
		}
		return uint(id), true
	case int64:
		if id < 0 {
			return 0, false
		}
		return uint(id), true
	case float64:
		if id < 0 {
			return 0, false
		}
		return uint(id), true
	default:
		return 0, false
	}
}
