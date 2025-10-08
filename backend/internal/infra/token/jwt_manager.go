/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 20:40:41
 * @FilePath: \electron-go-app\backend\internal\infra\token\jwt_manager.go
 * @LastEditTime: 2025-10-08 20:40:45
 */
package token

import (
	"context"
	"fmt"
	"time"

	domain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/service/auth"

	"github.com/golang-jwt/jwt/v5"
)

// JWTManager 基于对称加密密钥生成访问与刷新令牌。
type JWTManager struct {
	secret     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewJWTManager 创建 JWT 管理器，配置签名密钥以及访问/刷新令牌的有效期。
func NewJWTManager(secret string, accessTTL, refreshTTL time.Duration) *JWTManager {
	return &JWTManager{secret: secret, accessTTL: accessTTL, refreshTTL: refreshTTL}
}

// GenerateTokens 为指定用户签发访问令牌和刷新令牌，返回统一的 TokenPair。
func (m *JWTManager) GenerateTokens(ctx context.Context, user *domain.User) (auth.TokenPair, error) {
	accessToken, accessExp, err := m.buildToken(user, m.accessTTL)
	if err != nil {
		return auth.TokenPair{}, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, _, err := m.buildToken(user, m.refreshTTL)
	if err != nil {
		return auth.TokenPair{}, fmt.Errorf("generate refresh token: %w", err)
	}

	return auth.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(time.Until(accessExp).Seconds()),
	}, nil
}

// buildToken 根据指定 TTL 构造单个 JWT，包括基础 claims 与签名。
func (m *JWTManager) buildToken(user *domain.User, ttl time.Duration) (string, time.Time, error) {
	if ttl <= 0 {
		ttl = time.Hour
	}

	expiresAt := time.Now().Add(ttl)

	claims := jwt.MapClaims{
		"sub":      user.ID,
		"username": user.Username,
		"exp":      expiresAt.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(m.secret))
	if err != nil {
		return "", time.Time{}, err
	}

	return signed, expiresAt, nil
}
