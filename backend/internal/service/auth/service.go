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
	"gorm.io/gorm"
)

var (
	ErrEmailTaken           = errors.New("email already registered")
	ErrUsernameTaken        = errors.New("username already taken")
	ErrInvalidLogin         = errors.New("invalid email or password")
	ErrCaptchaRequired      = errors.New("captcha is required")
	ErrCaptchaInvalid       = errors.New("captcha verification failed")
	ErrCaptchaExpired       = errors.New("captcha expired or not found")
	ErrCaptchaRateLimited   = errors.New("captcha requests too frequent")
	ErrRefreshTokenInvalid  = errors.New("refresh token is invalid")
	ErrRefreshTokenExpired  = errors.New("refresh token expired")
	ErrRefreshTokenRevoked  = errors.New("refresh token revoked")
	ErrRefreshTokenRequired = errors.New("refresh token is required")
)

// CaptchaManager 聚合验证码生成与校验能力，便于在服务层替换实现。
type CaptchaManager interface {
	captcha.Generator
	captcha.Verifier
}

// TokenPair 表示一次鉴权流程中生成的访问令牌、刷新令牌及其过期时间。
// AccessToken 用于每次请求的身份校验；RefreshToken 用于续签新的 TokenPair。
// RefreshTokenID/RefreshTokenExpiresAt 是内部使用的元信息，帮助我们把刷新令牌写入存储并控制生命周期。
type TokenPair struct {
	AccessToken           string    `json:"access_token"`
	RefreshToken          string    `json:"refresh_token"`
	ExpiresIn             int64     `json:"expires_in"` // seconds
	RefreshTokenID        string    `json:"-"`
	RefreshTokenExpiresAt time.Time `json:"-"`
}

// TokenManager 抽象出签发 JWT 或其他令牌的能力，便于在不同实现之间切换。
// 目前仅有 JWTManager 一种实现。
type TokenManager interface {
	GenerateTokens(ctx context.Context, user *domain.User) (TokenPair, error)
	ParseRefreshToken(token string) (RefreshTokenClaims, error)
}

// RefreshTokenClaims 描述解析刷新令牌后得到的关键信息。
type RefreshTokenClaims struct {
	UserID    uint
	TokenID   string
	ExpiresAt time.Time
}

// RefreshTokenStore 负责存储和验证刷新令牌，用于登出和令牌续期。
type RefreshTokenStore interface {
	Save(ctx context.Context, userID uint, tokenID string, expiresAt time.Time) error
	Delete(ctx context.Context, userID uint, tokenID string) error
	Exists(ctx context.Context, userID uint, tokenID string) (bool, error)
}

// Service 负责处理用户注册、登录、刷新、登出等鉴权业务。
//
// 依赖说明：
//   - UserRepository：读写用户数据（注册、查询、更新时间）。
//   - TokenManager：生成 / 解析 access token 与 refresh token。
//   - RefreshTokenStore：保存刷新令牌的“指纹”（userID + jti），用于防止重复使用、实现登出。
//   - CaptchaManager：在注册时提供验证码校验能力，按需注入。
type Service struct {
	users        *repository.UserRepository
	tokenManager TokenManager
	logger       *zap.SugaredLogger
	captcha      CaptchaManager
	refreshStore RefreshTokenStore
}

// NewService 创建鉴权服务实例，并注入用户仓储与令牌管理器等核心依赖。
func NewService(users *repository.UserRepository, tm TokenManager, store RefreshTokenStore, cm CaptchaManager) *Service {
	baseLogger := appLogger.S().With("component", "auth.service")
	return &Service{users: users, tokenManager: tm, logger: baseLogger, captcha: cm, refreshStore: store}
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

// Register 完成注册流程：校验唯一性、校验验证码（若启用）、加密密码、持久化用户并签发 TokenPair。
// 返回值中的 TokenPair 会立即写入 RefreshTokenStore，从而允许用户在 access token 过期后无感续期。
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

	// 先确认邮箱是否占用。三种分支：找到 -> 直接返回 ErrEmailTaken；未找到 -> 继续；
	// 其他数据库错误 -> 立即中断并返回，避免在异常场景下继续注册。
	if _, err := s.users.FindByEmail(ctx, params.Email); err == nil {
		log.Warnw("email already registered")
		return nil, TokenPair{}, ErrEmailTaken
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorw("check email unique failed", "error", err)
		return nil, TokenPair{}, fmt.Errorf("check email unique: %w", err)
	}

	// 用户名校验沿用同样的三段式判断，确保真正的数据库错误不会被吞掉。
	if _, err := s.users.FindByUsername(ctx, params.Username); err == nil {
		log.Warnw("username already taken")
		return nil, TokenPair{}, ErrUsernameTaken
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorw("check username unique failed", "error", err)
		return nil, TokenPair{}, fmt.Errorf("check username unique: %w", err)
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

	tokens, err := s.issueAndStoreTokens(ctx, user)
	if err != nil {
		return nil, TokenPair{}, err
	}

	log.With("user_id", user.ID).Infow("user registered")

	return user, tokens, nil
}

// Login 校验用户凭证，更新登录时间，并重新签发新的 TokenPair。
// 当用户在多端登录时，最新的 refresh token 会覆盖旧的记录，使得每次登录都能获得“最新的一对令牌”。
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

	tokens, err := s.issueAndStoreTokens(ctx, user)
	if err != nil {
		return nil, TokenPair{}, err
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

// Refresh 使用刷新令牌换取新的访问令牌与刷新令牌。
//
// 适用场景举例：用户在页面上放置一段时间后重新操作，此时 access token 可能已过期；
// 前端会把仍然保留的 refresh token 发到该接口，期望“无感续命”。
//
// 链路说明：
//  1. 解析 refresh token -> 得到 userID、jti（令牌指纹）、过期时间。
//  2. 校验是否过期、是否格式错误；若刷新令牌也过期，则返回 ErrRefreshTokenExpired，前端需要重新登录。
//  3. 到 RefreshTokenStore 查 jti 是否存在，确保这张令牌没有被提前吊销或重复使用。
//  4. 删除旧 jti（实现“单次使用”），重新签发 access/refresh，并将新的 jti 写回存储。
//  5. 返回最新的 TokenPair，前端后续应替换本地缓存。
func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	log := s.scope("refresh")

	if strings.TrimSpace(refreshToken) == "" {
		log.Warn("missing refresh token")
		return TokenPair{}, ErrRefreshTokenRequired
	}

	claims, err := s.tokenManager.ParseRefreshToken(refreshToken)
	if err != nil {
		log.Warnw("parse refresh token failed", "error", err)
		return TokenPair{}, ErrRefreshTokenInvalid
	}

	if claims.ExpiresAt.IsZero() {
		log.Warn("refresh token missing expiry", "user_id", claims.UserID)
		return TokenPair{}, ErrRefreshTokenInvalid
	}

	if time.Now().After(claims.ExpiresAt) {
		log.Warn("refresh token expired", "user_id", claims.UserID)
		return TokenPair{}, ErrRefreshTokenExpired
	}

	if s.refreshStore == nil {
		log.Error("refresh store not configured")
		return TokenPair{}, fmt.Errorf("refresh token store missing")
	}

	ok, storeErr := s.refreshStore.Exists(ctx, claims.UserID, claims.TokenID)
	if storeErr != nil {
		log.Errorw("refresh store check failed", "error", storeErr)
		return TokenPair{}, fmt.Errorf("check refresh token: %w", storeErr)
	}
	if !ok {
		log.Warnw("refresh token revoked", "user_id", claims.UserID)
		return TokenPair{}, ErrRefreshTokenRevoked
	}

	user, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil {
		log.Errorw("load user failed", "error", err, "user_id", claims.UserID)
		return TokenPair{}, fmt.Errorf("load user: %w", err)
	}

	// 旋转刷新令牌：删除旧的，再生成新的。
	if err := s.refreshStore.Delete(ctx, claims.UserID, claims.TokenID); err != nil {
		log.Errorw("delete old refresh token failed", "error", err, "token_id", claims.TokenID)
		return TokenPair{}, fmt.Errorf("delete refresh token: %w", err)
	}

	tokens, issueErr := s.issueAndStoreTokens(ctx, user)
	if issueErr != nil {
		return TokenPair{}, issueErr
	}

	return tokens, nil
}

// Logout 撤销指定刷新令牌。
//
// 常见场景：用户主动退出登录、修改密码、被后台管理员强制下线等。
// 处理流程：解析 refresh token -> 获取 userID+jti -> 直接从存储层删除这条记录。
// 删除成功后，这张 refresh token 将无法再换取新的 access token，达到“彻底退出”的目的。
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	log := s.scope("logout")

	if strings.TrimSpace(refreshToken) == "" {
		log.Warn("missing refresh token")
		return ErrRefreshTokenRequired
	}

	claims, err := s.tokenManager.ParseRefreshToken(refreshToken)
	if err != nil {
		log.Warnw("parse refresh token failed", "error", err)
		return ErrRefreshTokenInvalid
	}

	if s.refreshStore == nil {
		log.Error("refresh store not configured")
		return fmt.Errorf("refresh token store missing")
	}

	if err := s.refreshStore.Delete(ctx, claims.UserID, claims.TokenID); err != nil {
		log.Errorw("delete refresh token failed", "error", err, "token_id", claims.TokenID)
		return fmt.Errorf("delete refresh token: %w", err)
	}

	return nil
}

// issueAndStoreTokens 是注册/登录/刷新等场景的公共步骤：
//  1. 调用 TokenManager 生成访问令牌 + 刷新令牌（含 jti、过期时间）。
//  2. 把刷新令牌的指纹写入 RefreshTokenStore，方便后续刷新、登出、吊销。
//  3. 返回 TokenPair 给调用方。
//
// 若保存刷新令牌失败，会把错误直接冒泡出去，确保不会返回“不可刷新”的令牌对。
func (s *Service) issueAndStoreTokens(ctx context.Context, user *domain.User) (TokenPair, error) {
	log := s.scope("issue_tokens").With("user_id", user.ID)

	tokens, err := s.tokenManager.GenerateTokens(ctx, user)
	if err != nil {
		log.Errorw("generate tokens failed", "error", err)
		return TokenPair{}, fmt.Errorf("generate tokens: %w", err)
	}

	if err := s.storeRefreshToken(ctx, user.ID, tokens); err != nil {
		return TokenPair{}, err
	}

	return tokens, nil
}

// storeRefreshToken 将刷新令牌指纹写入 RefreshTokenStore，供后续刷新/登出使用。
// 如果未注入 store（理论上不会发生），我们直接返回错误并阻止向客户端下发不可续期的 TokenPair。
func (s *Service) storeRefreshToken(ctx context.Context, userID uint, tokens TokenPair) error {
	if s.refreshStore == nil {
		return fmt.Errorf("refresh token store missing")
	}

	if tokens.RefreshTokenID == "" {
		return fmt.Errorf("refresh token id missing")
	}

	if tokens.RefreshTokenExpiresAt.IsZero() {
		return fmt.Errorf("refresh token expiry missing")
	}

	if err := s.refreshStore.Save(ctx, userID, tokens.RefreshTokenID, tokens.RefreshTokenExpiresAt); err != nil {
		s.scope("store_refresh").Errorw("save refresh token failed", "error", err, "user_id", userID)
		return fmt.Errorf("store refresh token: %w", err)
	}
	return nil
}
