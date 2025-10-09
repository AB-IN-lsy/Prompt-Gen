/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 20:40:06
 * @FilePath: \electron-go-app\backend\internal\service\auth\service.go
 * @LastEditTime: 2025-10-09 20:16:45
 */
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/infra/captcha"
	appLogger "electron-go-app/backend/internal/infra/logger"
	"electron-go-app/backend/internal/repository"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrUsernameTaken      = errors.New("username already taken")
	ErrInvalidLogin       = errors.New("invalid email or password")
	ErrCaptchaRequired    = errors.New("captcha is required")
	ErrCaptchaInvalid     = errors.New("captcha verification failed")
	ErrCaptchaExpired     = errors.New("captcha expired or not found")
	ErrCaptchaRateLimited = errors.New("captcha requests too frequent")
)

// CaptchaManager 聚合验证码生成与校验能力，便于在服务层替换实现。
type CaptchaManager interface {
	captcha.Generator
	captcha.Verifier
}

// TokenPair 表示一次鉴权流程中生成的访问令牌、刷新令牌及其过期时间。
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // seconds
}

// TokenManager 抽象出签发 JWT 或其他令牌的能力，便于在不同实现之间切换。
// 目前仅有 JWTManager 一种实现。
type TokenManager interface {
	GenerateTokens(ctx context.Context, user *domain.User) (TokenPair, error)
}

// Service 负责处理用户注册、登录等鉴权业务。
// 它依赖用户仓储用于持久化和查询数据，以及 TokenManager 用于生成令牌。
type Service struct {
	users        *repository.UserRepository
	tokenManager TokenManager
	logger       *zap.SugaredLogger
	captcha      CaptchaManager
}

// NewService 创建鉴权服务实例，并注入用户仓储与令牌管理器等核心依赖。
func NewService(users *repository.UserRepository, tm TokenManager, cm CaptchaManager) *Service {
	baseLogger := appLogger.S().With("component", "auth.service")
	return &Service{users: users, tokenManager: tm, logger: baseLogger, captcha: cm}
}

// RegisterParams 封装注册接口所需的输入参数。
type RegisterParams struct {
	Username    string
	Email       string
	Password    string
	CaptchaID   string
	CaptchaCode string
}

// LoginParams 封装登录接口所需的输入参数。
type LoginParams struct {
	Email    string
	Password string
}

// Register 完成注册流程：校验唯一性、加密密码、持久化用户并签发令牌。
func (s *Service) Register(ctx context.Context, params RegisterParams) (*domain.User, TokenPair, error) {
	log := s.scope("register").With(
		"email", params.Email,
		"username", params.Username,
	)

	log.Infow("register attempt")

	if s.captcha != nil {
		if strings.TrimSpace(params.CaptchaID) == "" || strings.TrimSpace(params.CaptchaCode) == "" {
			log.Warn("captcha required but missing")
			return nil, TokenPair{}, ErrCaptchaRequired
		}

		if err := s.captcha.Verify(ctx, params.CaptchaID, params.CaptchaCode); err != nil {
			switch {
			case errors.Is(err, captcha.ErrCaptchaNotFound):
				log.Warnw("captcha expired or not found", "captcha_id", params.CaptchaID)
				return nil, TokenPair{}, ErrCaptchaExpired
			case errors.Is(err, captcha.ErrCaptchaMismatch):
				log.Warnw("captcha mismatch", "captcha_id", params.CaptchaID)
				return nil, TokenPair{}, ErrCaptchaInvalid
			default:
				log.Errorw("captcha verify failed", "error", err)
				return nil, TokenPair{}, fmt.Errorf("captcha verify: %w", err)
			}
		}
	}

	if _, err := s.users.FindByEmail(ctx, params.Email); err == nil {
		log.Warnw("email already registered")
		return nil, TokenPair{}, ErrEmailTaken
	}

	if _, err := s.users.FindByUsername(ctx, params.Username); err == nil {
		log.Warnw("username already taken")
		return nil, TokenPair{}, ErrUsernameTaken
	}

	hash, err := hashPassword(params.Password)
	if err != nil {
		log.Errorw("hash password failed", "error", err)
		return nil, TokenPair{}, fmt.Errorf("hash password: %w", err)
	}

	settingsJSON, err := domain.SettingsJSON(domain.DefaultSettings())
	if err != nil {
		log.Errorw("encode default settings failed", "error", err)
		return nil, TokenPair{}, fmt.Errorf("default settings: %w", err)
	}

	user := &domain.User{
		Username:     params.Username,
		Email:        params.Email,
		PasswordHash: hash,
		Settings:     settingsJSON,
	}

	if err := s.users.Create(ctx, user); err != nil {
		log.Errorw("create user failed", "error", err)
		return nil, TokenPair{}, fmt.Errorf("create user: %w", err)
	}

	tokens, err := s.tokenManager.GenerateTokens(ctx, user)
	if err != nil {
		log.Errorw("generate tokens failed", "error", err, "user_id", user.ID)
		return nil, TokenPair{}, fmt.Errorf("generate tokens: %w", err)
	}

	log.With("user_id", user.ID).Infow("user registered")

	return user, tokens, nil
}

// Login 校验用户凭证，更新登录时间，并重新签发访问/刷新令牌。
func (s *Service) Login(ctx context.Context, params LoginParams) (*domain.User, TokenPair, error) {
	log := s.scope("login").With("email", params.Email)

	log.Infow("login attempt")

	user, err := s.users.FindByEmail(ctx, params.Email)
	if err != nil {
		log.Warnw("login email not found or repo error", "error", err)
		return nil, TokenPair{}, ErrInvalidLogin
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(params.Password)); err != nil {
		log.Warnw("password mismatch")
		return nil, TokenPair{}, ErrInvalidLogin
	}

	now := time.Now()
	user.LastLoginAt = &now
	if err := s.users.Update(ctx, user); err != nil {
		log.Errorw("update last login failed", "error", err, "user_id", user.ID)
		return nil, TokenPair{}, fmt.Errorf("update last login: %w", err)
	}

	tokens, err := s.tokenManager.GenerateTokens(ctx, user)
	if err != nil {
		log.Errorw("generate tokens failed", "error", err, "user_id", user.ID)
		return nil, TokenPair{}, fmt.Errorf("generate tokens: %w", err)
	}

	log.With("user_id", user.ID).Infow("login success")

	return user, tokens, nil
}

// hashPassword 使用 bcrypt 对明文密码加盐哈希，确保存储安全。
func hashPassword(password string) (string, error) {
	out, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (s *Service) ensureLogger() *zap.SugaredLogger {
	if s.logger == nil {
		s.logger = appLogger.S().With("component", "auth.service")
	}
	return s.logger
}

func (s *Service) scope(operation string) *zap.SugaredLogger {
	return s.ensureLogger().With("operation", operation)
}

// CaptchaEnabled 表示当前服务是否启用了验证码依赖。
func (s *Service) CaptchaEnabled() bool {
	return s != nil && s.captcha != nil
}

// GenerateCaptcha 调用底层验证码管理器生成图形验证码。
func (s *Service) GenerateCaptcha(ctx context.Context, ip string) (string, string, error) {
	if !s.CaptchaEnabled() {
		return "", "", ErrCaptchaRequired
	}

	id, b64, err := s.captcha.Generate(ctx, ip)
	if err != nil {
		if errors.Is(err, captcha.ErrRateLimited) {
			return "", "", ErrCaptchaRateLimited
		}
		return "", "", fmt.Errorf("generate captcha: %w", err)
	}

	return id, b64, nil
}
