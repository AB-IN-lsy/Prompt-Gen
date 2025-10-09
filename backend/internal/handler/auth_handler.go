/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 20:42:09
 * @FilePath: \electron-go-app\backend\internal\handler\auth_handler.go
 * @LastEditTime: 2025-10-08 21:15:23
 */
package handler

import (
	"net/http"
	"strings"

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
	Username    string `json:"username" binding:"required,min=3"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=6"`
	CaptchaID   string `json:"captcha_id"`
	CaptchaCode string `json:"captcha_code"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
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

	if h.service.CaptchaEnabled() {
		if strings.TrimSpace(req.CaptchaID) == "" || strings.TrimSpace(req.CaptchaCode) == "" {
			log.Warn("missing captcha fields")
			response.Fail(c, http.StatusBadRequest, response.ErrCaptchaRequired, "captcha is required", nil)
			return
		}
	}

	log.Infow("register request", "email", req.Email, "username", req.Username)

	user, tokens, err := h.service.Register(c.Request.Context(), auth.RegisterParams{
		Username:    req.Username,
		Email:       req.Email,
		Password:    req.Password,
		CaptchaID:   req.CaptchaID,
		CaptchaCode: req.CaptchaCode,
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
		case auth.ErrCaptchaRequired:
			status = http.StatusBadRequest
			code = response.ErrCaptchaRequired
			log.Warn("captcha required but missing")
		case auth.ErrCaptchaInvalid:
			status = http.StatusBadRequest
			code = response.ErrCaptchaInvalid
			log.Warn("captcha invalid")
		case auth.ErrCaptchaExpired:
			status = http.StatusBadRequest
			code = response.ErrCaptchaExpired
			log.Warn("captcha expired")
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

// Refresh 处理刷新令牌请求。
//
// 前端链路示例：页面检测到 401 或 access token 过期 -> 调用该接口并携带 refresh token ->
// 若成功，替换本地的 access/refresh token；若返回 401（刷新令牌过期/失效），提示用户重新登录。
//
// 该 Handler 只负责：
//  1. 解析请求体，确保 refresh_token 字段存在；
//  2. 调用 Service.Refresh，并根据不同错误类型映射为 400 / 401 / 500；
//  3. 将新的 TokenPair 返回给前端。
func (h *AuthHandler) Refresh(c *gin.Context) {
	log := h.scope("refresh")

	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnw("invalid request body", "error", err)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	tokens, err := h.service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		status := http.StatusInternalServerError
		code := response.ErrInternal
		switch err {
		case auth.ErrRefreshTokenRequired:
			status = http.StatusBadRequest
			code = response.ErrBadRequest
			log.Warn("missing refresh token")
		case auth.ErrRefreshTokenInvalid, auth.ErrRefreshTokenRevoked:
			status = http.StatusUnauthorized
			code = response.ErrUnauthorized
			log.Warn("refresh token invalid or revoked")
		case auth.ErrRefreshTokenExpired:
			status = http.StatusUnauthorized
			code = response.ErrUnauthorized
			log.Warn("refresh token expired")
		default:
			log.Errorw("refresh failed", "error", err)
		}
		response.Fail(c, status, code, err.Error(), nil)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"tokens": tokens}, nil)
}

// Logout 撤销刷新令牌。
//
// 链路示例：用户点击“退出登录”按钮 -> 前端发送持有的 refresh token ->
// 服务端调用 Service.Logout 删除存储记录 -> 返回 204，前端清理本地状态。
// 若刷新令牌已失效/格式错误，会返回 400/401，提示前端不用再尝试。
func (h *AuthHandler) Logout(c *gin.Context) {
	log := h.scope("logout")

	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnw("invalid request body", "error", err)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	if err := h.service.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		status := http.StatusInternalServerError
		code := response.ErrInternal
		switch err {
		case auth.ErrRefreshTokenRequired:
			status = http.StatusBadRequest
			code = response.ErrBadRequest
			log.Warn("missing refresh token")
		case auth.ErrRefreshTokenInvalid:
			status = http.StatusUnauthorized
			code = response.ErrUnauthorized
			log.Warn("refresh token invalid")
		default:
			log.Errorw("logout failed", "error", err)
		}
		response.Fail(c, status, code, err.Error(), nil)
		return
	}

	response.NoContent(c)
}

// Captcha 生成图形验证码并返回 base64 图片与标识。
func (h *AuthHandler) Captcha(c *gin.Context) {
	if !h.service.CaptchaEnabled() {
		response.Fail(c, http.StatusNotFound, response.ErrNotFound, "captcha not configured", nil)
		return
	}

	ip := c.ClientIP()
	id, img, err := h.service.GenerateCaptcha(c.Request.Context(), ip)
	if err != nil {
		status := http.StatusInternalServerError
		code := response.ErrInternal
		if err == auth.ErrCaptchaRateLimited {
			status = http.StatusTooManyRequests
			code = response.ErrTooManyRequests
		}
		h.scope("captcha").Errorw("generate captcha failed", "error", err, "ip", ip)
		response.Fail(c, status, code, err.Error(), nil)
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"captcha_id": id,
		"image":      img,
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
