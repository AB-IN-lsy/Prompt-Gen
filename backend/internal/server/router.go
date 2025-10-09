package server

import (
	"fmt"
	"strings"
	"time"

	"electron-go-app/backend/internal/handler"
	"electron-go-app/backend/internal/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type RouterOptions struct {
	AuthHandler   *handler.AuthHandler
	UserHandler   *handler.UserHandler
	UploadHandler *handler.UploadHandler
	AuthMW        *middleware.AuthMiddleware
}

func NewRouter(opts RouterOptions) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// gin 中间件配置
	r.Use(gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  false,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
		AllowOriginFunc: func(origin string) bool {
			if origin == "" {
				return false
			}
			if origin == "null" {
				return true
			}
			if strings.HasPrefix(origin, "http://localhost:") {
				return true
			}
			if strings.HasPrefix(origin, "http://127.0.0.1:") {
				return true
			}
			return false
		},
	}))
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

	r.Static("/static", "./public")

	api := r.Group("/api")
	{
		authGroup := api.Group("/auth")
		if opts.AuthHandler != nil {
			authGroup.GET("/captcha", opts.AuthHandler.Captcha)
			authGroup.POST("/register", opts.AuthHandler.Register)
			authGroup.POST("/login", opts.AuthHandler.Login)
			authGroup.POST("/refresh", opts.AuthHandler.Refresh)
			authGroup.POST("/logout", opts.AuthHandler.Logout)
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

		if opts.UploadHandler != nil {
			uploads := api.Group("/uploads")
			if opts.AuthMW != nil {
				uploads.Use(opts.AuthMW.Handle())
			}
			uploads.POST("/avatar", opts.UploadHandler.UploadAvatar)
		}
	}

	return r
}
