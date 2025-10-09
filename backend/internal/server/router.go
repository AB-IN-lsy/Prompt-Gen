package server

import (
	"fmt"
	"time"

	"electron-go-app/backend/internal/handler"
	"electron-go-app/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

type RouterOptions struct {
	AuthHandler *handler.AuthHandler
	UserHandler *handler.UserHandler
	AuthMW      *middleware.AuthMiddleware
}

func NewRouter(opts RouterOptions) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// gin 中间件配置
	r.Use(gin.Recovery())
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: gin.LogFormatter(func(params gin.LogFormatterParams) string {
			return fmt.Sprintf("%s - [%s] \"%s %s\" %d %s\n",
				params.ClientIP,
				params.TimeStamp.Format(time.RFC3339),
				params.Method,
				params.Path,
				params.StatusCode,
				params.Latency,
			)
		}),
	}))

	api := r.Group("/api")
	{
		authGroup := api.Group("/auth")
		{
			authGroup.GET("/captcha", opts.AuthHandler.Captcha)
			authGroup.POST("/register", opts.AuthHandler.Register)
			authGroup.POST("/login", opts.AuthHandler.Login)
		}

		// /api/users 下的路由需要登录才能访问，所以单独分组，再挂载 JWT 鉴权中间件。
		userGroup := api.Group("/users")
		if opts.AuthMW != nil {
			// Use 会把 AuthMiddleware.Handle() 返回的中间件插入到请求链中，确保先校验 JWT。
			userGroup.Use(opts.AuthMW.Handle())
		}
		if opts.UserHandler != nil {
			userGroup.GET("/me", opts.UserHandler.GetMe)
			userGroup.PUT("/me", opts.UserHandler.UpdateMe)
		}
	}

	return r
}
