/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 20:41:15
 * @FilePath: \electron-go-app\backend\internal\middleware\auth_middleware.go
 * @LastEditTime: 2025-10-08 20:41:20
 */
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware 基于共享密钥校验 JWT 的合法性，保护受限路由。
type AuthMiddleware struct {
	secret string
}

// NewAuthMiddleware 创建鉴权中间件实例，注入 JWT 签名密钥。
func NewAuthMiddleware(secret string) *AuthMiddleware {
	return &AuthMiddleware{secret: secret}
}

// Handle 返回 Gin 中间件，验证 Bearer Token 并在上下文中注入 claims。
func (m *AuthMiddleware) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		tokenString := strings.TrimSpace(authHeader[7:])
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrInvalidKeyType
			}
			return []byte(m.secret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}

		c.Set("claims", claims)
		c.Next()
	}
}
