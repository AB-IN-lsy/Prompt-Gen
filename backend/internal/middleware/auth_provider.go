package middleware

import "github.com/gin-gonic/gin"

// Authenticator 抽象鉴权中间件，实现 Handle() 的结构体即可插入路由。
type Authenticator interface {
	Handle() gin.HandlerFunc
}
