/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 20:40:41
 * @FilePath: \electron-go-app\backend\internal\infra\token\jwt_manager.go
 * @LastEditTime: 2025-10-08 23:05:40
 */
package token

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	domain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/service/auth"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	claimTokenType   = "token_type"
	claimTokenID     = "jti"
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
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
	accessToken, accessExp, _, err := m.buildToken(user, m.accessTTL, tokenTypeAccess, "")
	if err != nil {
		return auth.TokenPair{}, fmt.Errorf("generate access token: %w", err)
	}

	refreshID := uuid.NewString()
	refreshToken, refreshExp, refreshID, err := m.buildToken(user, m.refreshTTL, tokenTypeRefresh, refreshID)
	if err != nil {
		return auth.TokenPair{}, fmt.Errorf("generate refresh token: %w", err)
	}

	return auth.TokenPair{
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		ExpiresIn:             int64(time.Until(accessExp).Seconds()),
		RefreshTokenID:        refreshID,
		RefreshTokenExpiresAt: refreshExp,
	}, nil
}

// buildToken 根据指定 TTL 构造单个 JWT，包括基础 claims 与签名。
func (m *JWTManager) buildToken(user *domain.User, ttl time.Duration, tokenType string, tokenID string) (string, time.Time, string, error) {
	if ttl <= 0 {
		ttl = time.Hour
	}

	expiresAt := time.Now().Add(ttl)

	// 这里使用 MapClaims，方便后续扩展自定义字段。
	// 常见的标准字段包括 iss（签发者）、sub（主题）、aud（受众）、exp（过期时间）等。
	claims := jwt.MapClaims{
		"sub":          fmt.Sprintf("%d", user.ID),
		"username":     user.Username,
		"exp":          expiresAt.Unix(),
		"is_admin":     user.IsAdmin,
		claimTokenType: tokenType,
	}

	if tokenType == tokenTypeRefresh {
		if tokenID == "" {
			tokenID = uuid.NewString()
		}
		claims[claimTokenID] = tokenID
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(m.secret))
	if err != nil {
		return "", time.Time{}, "", err
	}

	return signed, expiresAt, tokenID, nil
}

// ParseRefreshToken 验证并解析刷新令牌，返回其包含的用户 ID 与 TokenID。
func (m *JWTManager) ParseRefreshToken(raw string) (auth.RefreshTokenClaims, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrInvalidKeyType
		}
		return []byte(m.secret), nil
	})
	if err != nil {
		return auth.RefreshTokenClaims{}, err
	}
	if !token.Valid {
		return auth.RefreshTokenClaims{}, errors.New("token invalid")
	}

	tType, _ := claims[claimTokenType].(string)
	if tType != tokenTypeRefresh {
		return auth.RefreshTokenClaims{}, errors.New("not a refresh token")
	}

	var subRaw string
	switch v := claims["sub"].(type) {
	case string:
		subRaw = v
	case float64:
		if v < 0 {
			return auth.RefreshTokenClaims{}, errors.New("invalid subject")
		}
		subRaw = fmt.Sprintf("%.0f", v)
	case json.Number:
		subRaw = v.String()
	default:
		return auth.RefreshTokenClaims{}, errors.New("missing subject")
	}

	id64, err := strconv.ParseUint(subRaw, 10, 64)
	if err != nil {
		return auth.RefreshTokenClaims{}, fmt.Errorf("parse subject: %w", err)
	}

	tokenID, _ := claims[claimTokenID].(string)
	if tokenID == "" {
		return auth.RefreshTokenClaims{}, errors.New("missing refresh token id")
	}

	var expiresAt time.Time
	switch expVal := claims["exp"].(type) {
	case float64:
		expiresAt = time.Unix(int64(expVal), 0)
	case json.Number:
		if v, err := expVal.Int64(); err == nil {
			expiresAt = time.Unix(v, 0)
		}
	case string:
		if v, err := strconv.ParseInt(expVal, 10, 64); err == nil {
			expiresAt = time.Unix(v, 0)
		}
	}

	return auth.RefreshTokenClaims{
		UserID:    uint(id64),
		TokenID:   tokenID,
		ExpiresAt: expiresAt,
	}, nil
}
