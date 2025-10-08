/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 22:38:26
 * @FilePath: \electron-go-app\backend\internal\handler\user_handler.go
 * @LastEditTime: 2025-10-08 22:38:31
 */
package handler

import (
	"net/http"

	usersvc "electron-go-app/backend/internal/service/user"

	"github.com/gin-gonic/gin"
)

// UserHandler 负责用户资料相关的 HTTP 入口。
type UserHandler struct {
	service *usersvc.Service
}

// NewUserHandler 构造用户 handler。
func NewUserHandler(service *usersvc.Service) *UserHandler {
	return &UserHandler{service: service}
}

// GetMe 返回当前登录用户资料。
func (h *UserHandler) GetMe(c *gin.Context) {
	userID, ok := extractUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	profile, err := h.service.GetProfile(c.Request.Context(), userID)
	if err != nil {
		status := http.StatusInternalServerError
		if err == usersvc.ErrUserNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// UpdateSettingsRequest 描述更新设置的请求体。
type UpdateSettingsRequest struct {
	PreferredModel string `json:"preferred_model" binding:"omitempty,min=1"`
	SyncEnabled    *bool  `json:"sync_enabled"`
}

// UpdateMe 更新当前登录用户的设置。
func (h *UserHandler) UpdateMe(c *gin.Context) {
	userID, ok := extractUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	profile, err := h.service.GetProfile(c.Request.Context(), userID)
	if err != nil {
		status := http.StatusInternalServerError
		if err == usersvc.ErrUserNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
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
		if err == usersvc.ErrUserNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated)
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
