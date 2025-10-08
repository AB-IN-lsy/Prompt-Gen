/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 20:41:15
 * @FilePath: \electron-go-app\backend\internal\middleware\auth_middleware.go
 * @LastEditTime: 2025-10-08 20:41:20
 */
package middleware

import (
	"net/http"
	"strconv"
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

		// claims 就是 JWT payload 中的键值对（MapClaims），常见字段包括 iss、exp、sub 等。
		// 这里把它保存下来，后续 handler 可以根据需要取更多信息。
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}
		c.Set("claims", claims)

		// JWT 里的 sub（subject）通常存的是用户 ID。为了后续业务方便，这里尝试解析成数字，
		// 并写入 Gin Context，路由中的 handler 可以直接通过 c.Get("userID") 取到当前登录用户。
		if sub, ok := claims["sub"]; ok {
			switch v := sub.(type) {
			case string:
				if id, err := strconv.ParseUint(v, 10, 64); err == nil {
					c.Set("userID", uint(id))
				}
			case float64:
				if v >= 0 {
					c.Set("userID", uint(v))
				}
			}
		}
		c.Next()
	}
}
