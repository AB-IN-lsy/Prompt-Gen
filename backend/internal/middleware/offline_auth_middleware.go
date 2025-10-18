package middleware

import "github.com/gin-gonic/gin"

// OfflineAuthMiddleware 在本地模式下注入固定用户，绕过 JWT 校验流程。
type OfflineAuthMiddleware struct {
	userID  uint
	isAdmin bool
}

// NewOfflineAuthMiddleware 构造用于离线模式的鉴权中间件。
func NewOfflineAuthMiddleware(userID uint, isAdmin bool) *OfflineAuthMiddleware {
	return &OfflineAuthMiddleware{
		userID:  userID,
		isAdmin: isAdmin,
	}
}

// Handle 将固定用户写入上下文，使后续 Handler 可以读取 userID/isAdmin。
func (m *OfflineAuthMiddleware) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("userID", m.userID)
		c.Set("isAdmin", m.isAdmin)
		c.Next()
	}
}
