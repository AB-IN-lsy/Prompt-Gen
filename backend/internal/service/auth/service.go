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

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrEmailTaken               = errors.New("email already registered")
	ErrUsernameTaken            = errors.New("username already taken")
	ErrEmailAndUsernameTaken    = errors.New("email and username already taken")
	ErrInvalidLogin             = errors.New("invalid email or password")
	ErrEmailNotVerified         = errors.New("email not verified")
	ErrVerificationTokenInvalid = errors.New("verification token is invalid or expired")
	ErrEmailAlreadyVerified     = errors.New("email already verified")
	ErrVerificationNotEnabled   = errors.New("email verification store not configured")
	ErrCaptchaRequired          = errors.New("captcha is required")
	ErrCaptchaInvalid           = errors.New("captcha verification failed")
	ErrCaptchaExpired           = errors.New("captcha expired or not found")
	ErrCaptchaRateLimited       = errors.New("captcha requests too frequent")
	ErrRefreshTokenInvalid      = errors.New("refresh token is invalid")
	ErrRefreshTokenExpired      = errors.New("refresh token expired")
	ErrRefreshTokenRevoked      = errors.New("refresh token revoked")
	ErrRefreshTokenRequired     = errors.New("refresh token is required")
)

const emailVerificationTTL = 24 * time.Hour

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

// EmailSender 定义发送验证邮件的能力，便于后续替换为真实邮件服务。
type EmailSender interface {
	SendVerification(ctx context.Context, user *domain.User, token string) error
}

type loggingEmailSender struct {
	logger *zap.SugaredLogger
}

func (l *loggingEmailSender) SendVerification(ctx context.Context, user *domain.User, token string) error {
	if l == nil {
		return nil
	}
	l.logger.Infow("email verification token issued", "user_id", user.ID, "email", user.Email, "token", token)
	return nil
}

// Service 负责处理用户注册、登录、刷新、登出等鉴权业务。
//
// 依赖说明：
//   - UserRepository：读写用户数据（注册、查询、更新时间）。
//   - TokenManager：生成 / 解析 access token 与 refresh token。
//   - RefreshTokenStore：保存刷新令牌的“指纹”（userID + jti），用于防止重复使用、实现登出。
//   - CaptchaManager：在注册时提供验证码校验能力，按需注入。
type Service struct {
	users         *repository.UserRepository
	verifications *repository.EmailVerificationRepository
	tokenManager  TokenManager
	logger        *zap.SugaredLogger
	captcha       CaptchaManager
	refreshStore  RefreshTokenStore
	emailSender   EmailSender
}

// NewService 创建鉴权服务实例，并注入用户仓储与令牌管理器等核心依赖。
func NewService(users *repository.UserRepository, verifications *repository.EmailVerificationRepository, tm TokenManager, store RefreshTokenStore, cm CaptchaManager, sender EmailSender) *Service {
	baseLogger := appLogger.S().With("component", "auth.service")
	if sender == nil {
		sender = &loggingEmailSender{logger: baseLogger}
	}
	return &Service{
		users:         users,
		verifications: verifications,
		tokenManager:  tm,
		logger:        baseLogger,
		captcha:       cm,
		refreshStore:  store,
		emailSender:   sender,
	}
}

// RegisterParams 封装注册接口所需的输入参数。
type RegisterParams struct {
	Username    string
	Email       string
	Password    string
	AvatarURL   string
	CaptchaID   string
	CaptchaCode string
}

// LoginParams 封装登录接口所需的输入参数。
type LoginParams struct {
	Identifier string
	Password   string
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

	// 先确认邮箱/用户名是否占用：若任一字段在库中已存在，记录标记，稍后统一返回。
	emailTaken := false
	if _, err := s.users.FindByEmail(ctx, params.Email); err == nil {
		emailTaken = true
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorw("check email unique failed", "error", err)
		return nil, TokenPair{}, fmt.Errorf("check email unique: %w", err)
	}

	usernameTaken := false
	if _, err := s.users.FindByUsername(ctx, params.Username); err == nil {
		usernameTaken = true
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorw("check username unique failed", "error", err)
		return nil, TokenPair{}, fmt.Errorf("check username unique: %w", err)
	}

	if emailTaken && usernameTaken {
		log.Warnw("email and username already taken", "email", params.Email, "username", params.Username)
		return nil, TokenPair{}, ErrEmailAndUsernameTaken
	}

	if emailTaken {
		log.Warnw("email already registered")
		return nil, TokenPair{}, ErrEmailTaken
	}

	if usernameTaken {
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

	avatar := strings.TrimSpace(params.AvatarURL)

	user := &domain.User{
		Username:     params.Username,
		Email:        params.Email,
		AvatarURL:    avatar,
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

	if user.EmailVerifiedAt == nil {
		if _, err := s.issueEmailVerificationToken(ctx, user); err != nil {
			log.Warnw("issue email verification token failed", "error", err)
		}
	}

	log.With("user_id", user.ID).Infow("user registered")

	return user, tokens, nil
}

// Login 校验用户凭证（支持邮箱或用户名），更新登录时间，并重新签发新的 TokenPair。
// 当用户在多端登录时，最新的 refresh token 会覆盖旧的记录，使得每次登录都能获得“最新的一对令牌”。
func (s *Service) Login(ctx context.Context, params LoginParams) (*domain.User, TokenPair, error) {
	identifier := strings.TrimSpace(params.Identifier)
	log := s.scope("login").With("identifier", identifier)

	log.Infow("login attempt")

	var (
		user *domain.User
		err  error
	)

	if strings.Contains(identifier, "@") {
		user, err = s.users.FindByEmail(ctx, identifier)
	} else {
		user, err = s.users.FindByUsername(ctx, identifier)
	}

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 尝试另一种方式查找：先按 email，失败再按用户名（或反之）。
			if strings.Contains(identifier, "@") {
				user, err = s.users.FindByUsername(ctx, identifier)
			} else {
				user, err = s.users.FindByEmail(ctx, identifier)
			}
		}
	}

	if err != nil {
		log.Warnw("login identifier not found or repo error", "error", err)
		return nil, TokenPair{}, ErrInvalidLogin
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(params.Password)); err != nil {
		log.Warnw("password mismatch")
		return nil, TokenPair{}, ErrInvalidLogin
	}

	if user.EmailVerifiedAt == nil {
		// 没有验证过邮箱的账号被禁止登录，要求先完成邮件确认流程。
		log.Warnw("email not verified", "user_id", user.ID)
		return nil, TokenPair{}, ErrEmailNotVerified
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

// RequestEmailVerification 会为尚未完成验证的用户重新生成验证码。
// 常用于“没收到邮件”或主动点击“重新发送”场景。
func (s *Service) RequestEmailVerification(ctx context.Context, email string) (string, error) {
	if s.verifications == nil {
		return "", ErrVerificationNotEnabled
	}

	user, err := s.users.FindByEmail(ctx, strings.TrimSpace(email))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrInvalidLogin
		}
		return "", fmt.Errorf("find user: %w", err)
	}

	if user.EmailVerifiedAt != nil {
		return "", ErrEmailAlreadyVerified
	}

	token, err := s.issueEmailVerificationToken(ctx, user)
	if err != nil {
		return "", err
	}

	return token, nil
}

// VerifyEmail 接收来自邮件的 token，校验其有效性，然后将用户标记为“已验证”。
// 成功后会立即将 token 标记为已消费，防止重复使用。
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	if s.verifications == nil {
		return ErrVerificationNotEnabled
	}

	record, err := s.verifications.FindValidToken(ctx, strings.TrimSpace(token))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrVerificationTokenInvalid
		}
		return fmt.Errorf("lookup verification token: %w", err)
	}

	if record == nil {
		return ErrVerificationTokenInvalid
	}

	now := time.Now()
	if err := s.users.MarkEmailVerified(ctx, record.UserID, now); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrVerificationTokenInvalid
		}
		return fmt.Errorf("mark verified: %w", err)
	}

	if err := s.verifications.MarkConsumed(ctx, record.ID); err != nil {
		return fmt.Errorf("consume verification token: %w", err)
	}

	s.scope("verify_email").Infow("email verified", "user_id", record.UserID)
	return nil
}

// issueEmailVerificationToken 根据用户生成新的 UUID 令牌，并写入数据库。
// 若用户已经有旧 token，会先删除旧记录再写入新 token。
// 最后调用 EmailSender（默认打印日志，可替换为实际邮件服务）通知用户。
func (s *Service) issueEmailVerificationToken(ctx context.Context, user *domain.User) (string, error) {
	if s.verifications == nil {
		return "", ErrVerificationNotEnabled
	}

	token := uuid.NewString()
	record := &domain.EmailVerificationToken{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(emailVerificationTTL),
	}

	if err := s.verifications.UpsertToken(ctx, record); err != nil {
		return "", fmt.Errorf("save verification token: %w", err)
	}

	go func() {
		if _, err := s.verifications.DeleteExpired(context.Background(), time.Now()); err != nil {
			s.scope("verify_email_cleanup").Warnw("delete expired tokens failed", "error", err)
		}
	}()

	if err := s.emailSender.SendVerification(ctx, user, token); err != nil {
		s.scope("verify_email").Warnw("send verification email failed", "error", err, "user_id", user.ID)
	}

	return token, nil
}

// CaptchaEnabled 表示当前服务是否启用了验证码依赖。
func (s *Service) CaptchaEnabled() bool {
	return s != nil && s.captcha != nil
}

// GenerateCaptcha 调用底层验证码管理器生成图形验证码。
func (s *Service) GenerateCaptcha(ctx context.Context, ip string) (string, string, int, error) {
	if !s.CaptchaEnabled() {
		return "", "", 0, ErrCaptchaRequired
	}

	id, b64, remaining, err := s.captcha.Generate(ctx, ip)
	if err != nil {
		if errors.Is(err, captcha.ErrRateLimited) {
			return "", "", 0, ErrCaptchaRateLimited
		}
		return "", "", 0, fmt.Errorf("generate captcha: %w", err)
	}

	return id, b64, remaining, nil
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
