/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 20:41:15
 * @FilePath: \electron-go-app\backend\internal\middleware\auth_middleware.go
 * @LastEditTime: 2025-10-09 19:36:01
 */
package middleware

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	response "electron-go-app/backend/internal/infra/common"

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
			fmt.Println("[auth-middleware] missing authorization header, rejecting request")
			response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing authorization header", nil)
			c.Abort()
			return
		}

		tokenString := strings.TrimSpace(authHeader[7:])
		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrInvalidKeyType
			}
			return []byte(m.secret), nil
		})
		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "token expired", nil)
			} else {
				response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "invalid token", err.Error())
			}
			c.Abort()
			return
		}
		if !token.Valid {
			response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "invalid token", nil)
			c.Abort()
			return
		}

		if expVal, ok := claims["exp"]; ok {
			var exp int64
			switch v := expVal.(type) {
			case float64:
				exp = int64(v)
			case json.Number:
				if parsed, err := v.Int64(); err == nil {
					exp = parsed
				}
			case string:
				if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
					exp = parsed
				}
			}
			if exp != 0 && time.Now().Unix() >= exp {
				response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "token expired", nil)
				c.Abort()
				return
			}
		}

		c.Set("claims", claims)

		if raw, ok := claims["is_admin"]; ok {
			switch v := raw.(type) {
			case bool:
				c.Set("isAdmin", v)
			case string:
				c.Set("isAdmin", strings.EqualFold(v, "true") || v == "1")
			case float64:
				c.Set("isAdmin", v != 0)
			default:
				c.Set("isAdmin", false)
			}
		} else {
			c.Set("isAdmin", false)
		}

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
