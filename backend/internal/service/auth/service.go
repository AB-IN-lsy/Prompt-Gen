/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 20:40:06
 * @FilePath: \electron-go-app\backend\internal\service\auth\service.go
 * @LastEditTime: 2025-10-08 20:40:11
 */
package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	domain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailTaken    = errors.New("email already registered")
	ErrUsernameTaken = errors.New("username already taken")
	ErrInvalidLogin  = errors.New("invalid email or password")
)

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
}

// NewService 创建鉴权服务实例，并注入用户仓储与令牌管理器等核心依赖。
func NewService(users *repository.UserRepository, tm TokenManager) *Service {
	return &Service{users: users, tokenManager: tm}
}

// RegisterParams 封装注册接口所需的输入参数。
type RegisterParams struct {
	Username string
	Email    string
	Password string
}

// LoginParams 封装登录接口所需的输入参数。
type LoginParams struct {
	Email    string
	Password string
}

// Register 完成注册流程：校验唯一性、加密密码、持久化用户并签发令牌。
func (s *Service) Register(ctx context.Context, params RegisterParams) (*domain.User, TokenPair, error) {
	if _, err := s.users.FindByEmail(ctx, params.Email); err == nil {
		return nil, TokenPair{}, ErrEmailTaken
	}

	if _, err := s.users.FindByUsername(ctx, params.Username); err == nil {
		return nil, TokenPair{}, ErrUsernameTaken
	}

	hash, err := hashPassword(params.Password)
	if err != nil {
		return nil, TokenPair{}, fmt.Errorf("hash password: %w", err)
	}

	user := &domain.User{
		Username:     params.Username,
		Email:        params.Email,
		PasswordHash: hash,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return nil, TokenPair{}, fmt.Errorf("create user: %w", err)
	}

	tokens, err := s.tokenManager.GenerateTokens(ctx, user)
	if err != nil {
		return nil, TokenPair{}, fmt.Errorf("generate tokens: %w", err)
	}

	return user, tokens, nil
}

// Login 校验用户凭证，更新登录时间，并重新签发访问/刷新令牌。
func (s *Service) Login(ctx context.Context, params LoginParams) (*domain.User, TokenPair, error) {
	user, err := s.users.FindByEmail(ctx, params.Email)
	if err != nil {
		return nil, TokenPair{}, ErrInvalidLogin
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(params.Password)); err != nil {
		return nil, TokenPair{}, ErrInvalidLogin
	}

	now := time.Now()
	user.LastLoginAt = &now
	if err := s.users.Update(ctx, user); err != nil {
		return nil, TokenPair{}, fmt.Errorf("update last login: %w", err)
	}

	tokens, err := s.tokenManager.GenerateTokens(ctx, user)
	if err != nil {
		return nil, TokenPair{}, fmt.Errorf("generate tokens: %w", err)
	}

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
