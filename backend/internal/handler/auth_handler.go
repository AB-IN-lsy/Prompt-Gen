/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 20:42:09
 * @FilePath: \electron-go-app\backend\internal\handler\auth_handler.go
 * @LastEditTime: 2025-10-08 21:15:23
 */
package handler

import (
	"net/http"

	"electron-go-app/backend/internal/service/auth"

	"github.com/gin-gonic/gin"
)

// AuthHandler 负责对接 Gin，处理鉴权相关的 HTTP 请求。
type AuthHandler struct {
	service *auth.Service
}

// NewAuthHandler 构造鉴权 handler，注入业务层服务做实际处理。
func NewAuthHandler(service *auth.Service) *AuthHandler {
	return &AuthHandler{service: service}
}

type registerRequest struct {
	Username string `json:"username" binding:"required,min=3"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Register 处理用户注册的 HTTP 请求，验证参数并调用业务逻辑。
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, tokens, err := h.service.Register(c.Request.Context(), auth.RegisterParams{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		status := http.StatusInternalServerError
		switch err {
		case auth.ErrEmailTaken:
			status = http.StatusConflict
		case auth.ErrUsernameTaken:
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user":   user,
		"tokens": tokens,
	})
}

// Login 处理用户登录请求，校验凭证并返回令牌。
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, tokens, err := h.service.Login(c.Request.Context(), auth.LoginParams{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if err == auth.ErrInvalidLogin {
			status = http.StatusUnauthorized
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user":   user,
		"tokens": tokens,
	})
}
