/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 22:38:26
 * @FilePath: \electron-go-app\backend\internal\handler\user_handler.go
 * @LastEditTime: 2025-10-10 02:42:06
 */
package handler

import (
	"net/http"
	"strings"

	response "electron-go-app/backend/internal/infra/common"
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

// UpdateMeRequest 描述更新当前登录用户资料与设置的请求体。
type UpdateMeRequest struct {
	Username       *string `json:"username" binding:"omitempty,min=2,max=64"`
	Email          *string `json:"email" binding:"omitempty,email"`
	AvatarURL      *string `json:"avatar_url"`
	PreferredModel string  `json:"preferred_model" binding:"omitempty,min=1"`
	SyncEnabled    *bool   `json:"sync_enabled"`
}

// UpdateMe 更新当前登录用户的设置与基础信息。
func (h *UserHandler) UpdateMe(c *gin.Context) {
	log := h.scope("update_me")

	userID, ok := extractUserID(c)
	if !ok {
		log.Warnw("missing user id")
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}

	log = log.With("user_id", userID)

	var req UpdateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnw("invalid request body", "error", err)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	usernameLog := ""
	if req.Username != nil {
		usernameLog = strings.TrimSpace(*req.Username)
	}
	emailLog := ""
	if req.Email != nil {
		emailLog = strings.TrimSpace(*req.Email)
	}
	avatarProvided := req.AvatarURL != nil

	log.Infow("update request", "username", usernameLog, "email", emailLog, "avatar_provided", avatarProvided, "preferred_model", req.PreferredModel, "sync_enabled", req.SyncEnabled)

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

	current := profile

	profileUpdates := usersvc.UpdateProfileParams{}
	if req.Username != nil {
		username := strings.TrimSpace(*req.Username)
		if username == "" {
			response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "username cannot be empty", gin.H{"field": "username"})
			return
		}
		profileUpdates.Username = &username
	}
	if req.Email != nil {
		email := strings.TrimSpace(*req.Email)
		if email == "" {
			response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "email cannot be empty", gin.H{"field": "email"})
			return
		}
		profileUpdates.Email = &email
	}
	if req.AvatarURL != nil {
		avatar := strings.TrimSpace(*req.AvatarURL)
		profileUpdates.AvatarURL = &avatar
	}

	if profileUpdates.Username != nil || profileUpdates.Email != nil || profileUpdates.AvatarURL != nil {
		updatedProfile, updateErr := h.service.UpdateProfile(c.Request.Context(), userID, profileUpdates)
		if updateErr != nil {
			status := http.StatusInternalServerError
			code := response.ErrInternal
			var details gin.H
			switch updateErr {
			case usersvc.ErrUserNotFound:
				status = http.StatusNotFound
				code = response.ErrNotFound
				log.Warnw("user not found on profile update")
			case usersvc.ErrEmailTaken:
				status = http.StatusConflict
				code = response.ErrConflict
				details = gin.H{"field": "email"}
				log.Warnw("profile update conflict", "error", updateErr)
			case usersvc.ErrUsernameTaken:
				status = http.StatusConflict
				code = response.ErrConflict
				details = gin.H{"field": "username"}
				log.Warnw("profile update conflict", "error", updateErr)
			default:
				log.Errorw("update profile failed", "error", updateErr)
			}
			response.Fail(c, status, code, updateErr.Error(), details)
			return
		}
		current = updatedProfile
	}

	settings := current.Settings
	if req.PreferredModel != "" {
		settings.PreferredModel = req.PreferredModel
	}
	if req.SyncEnabled != nil {
		settings.SyncEnabled = *req.SyncEnabled
	}

	if req.PreferredModel != "" || req.SyncEnabled != nil {
		updatedSettings, updateErr := h.service.UpdateSettings(c.Request.Context(), userID, settings)
		if updateErr != nil {
			status := http.StatusInternalServerError
			code := response.ErrInternal
			var details gin.H
			if updateErr == usersvc.ErrUserNotFound {
				status = http.StatusNotFound
				code = response.ErrNotFound
				log.Warnw("user not found on settings update")
			} else if updateErr == usersvc.ErrPreferredModelNotFound {
				status = http.StatusBadRequest
				code = response.ErrBadRequest
				details = gin.H{"field": "preferred_model"}
				log.Warnw("preferred model not found", "preferred_model", req.PreferredModel)
			} else if updateErr == usersvc.ErrPreferredModelDisabled {
				status = http.StatusBadRequest
				code = response.ErrBadRequest
				details = gin.H{"field": "preferred_model"}
				log.Warnw("preferred model disabled", "preferred_model", req.PreferredModel)
			} else {
				log.Errorw("update settings failed", "error", updateErr)
			}
			response.Fail(c, status, code, updateErr.Error(), details)
			return
		}
		current = updatedSettings
	}

	log.Infow("update success", "username", current.User.Username, "email", current.User.Email, "preferred_model", current.Settings.PreferredModel, "sync_enabled", current.Settings.SyncEnabled)

	response.Success(c, http.StatusOK, current, nil)
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

func isAdmin(c *gin.Context) bool {
	val, ok := c.Get("isAdmin")
	if !ok {
		return false
	}
	if b, ok := val.(bool); ok {
		return b
	}
	return false
}
