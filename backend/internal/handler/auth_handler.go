/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 20:42:09
 * @FilePath: \electron-go-app\backend\internal\handler\auth_handler.go
 * @LastEditTime: 2025-10-08 21:15:23
 */
package handler

import (
	"net/http"

	response "electron-go-app/backend/internal/infra/common"
	appLogger "electron-go-app/backend/internal/infra/logger"
	"electron-go-app/backend/internal/service/auth"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AuthHandler 负责对接 Gin，处理鉴权相关的 HTTP 请求。
type AuthHandler struct {
	service *auth.Service
	logger  *zap.SugaredLogger
}

// NewAuthHandler 构造鉴权 handler，注入业务层服务做实际处理。
func NewAuthHandler(service *auth.Service) *AuthHandler {
	baseLogger := appLogger.S().With("component", "auth.handler")
	return &AuthHandler{service: service, logger: baseLogger}
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
	log := h.scope("register")

	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnw("invalid request body", "error", err)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	log.Infow("register request", "email", req.Email, "username", req.Username)

	user, tokens, err := h.service.Register(c.Request.Context(), auth.RegisterParams{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		status := http.StatusInternalServerError
		code := response.ErrInternal
		switch err {
		case auth.ErrEmailTaken:
			status = http.StatusConflict
			code = response.ErrConflict
			log.Warnw("email already taken", "email", req.Email)
		case auth.ErrUsernameTaken:
			status = http.StatusConflict
			code = response.ErrConflict
			log.Warnw("username already taken", "username", req.Username)
		default:
			log.Errorw("register failed", "error", err)
		}
		response.Fail(c, status, code, err.Error(), nil)
		return
	}

	log.Infow("register success", "user_id", user.ID)

	response.Created(c, gin.H{
		"user":   user,
		"tokens": tokens,
	}, nil)
}

// Login 处理用户登录请求，校验凭证并返回令牌。
func (h *AuthHandler) Login(c *gin.Context) {
	log := h.scope("login")

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnw("invalid request body", "error", err)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	log.Infow("login request", "email", req.Email)

	user, tokens, err := h.service.Login(c.Request.Context(), auth.LoginParams{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		status := http.StatusInternalServerError
		code := response.ErrInternal
		if err == auth.ErrInvalidLogin {
			status = http.StatusUnauthorized
			code = response.ErrUnauthorized
			log.Warnw("login failed: invalid credential", "email", req.Email)
		} else {
			log.Errorw("login failed", "error", err, "email", req.Email)
		}
		response.Fail(c, status, code, err.Error(), nil)
		return
	}

	log.Infow("login success", "user_id", user.ID)

	response.Success(c, http.StatusOK, gin.H{
		"user":   user,
		"tokens": tokens,
	}, nil)
}

func (h *AuthHandler) ensureLogger() *zap.SugaredLogger {
	if h.logger == nil {
		h.logger = appLogger.S().With("component", "auth.handler")
	}
	return h.logger
}

func (h *AuthHandler) scope(operation string) *zap.SugaredLogger {
	return h.ensureLogger().With("operation", operation)
}
