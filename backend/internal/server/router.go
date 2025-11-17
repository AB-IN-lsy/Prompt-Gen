package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"electron-go-app/backend/internal/handler"
	"electron-go-app/backend/internal/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type RouterOptions struct {
	AuthHandler          *handler.AuthHandler
	UserHandler          *handler.UserHandler
	UploadHandler        *handler.UploadHandler
	ModelHandler         *handler.ModelHandler
	ChangelogHandler     *handler.ChangelogHandler
	AdminMetricsHandler  *handler.AdminMetricsHandler
	AdminUserHandler     *handler.AdminUserHandler
	PromptHandler        *handler.PromptHandler
	PromptCommentHandler *handler.PromptCommentHandler
	PublicPromptHandler  *handler.PublicPromptHandler
	AuthMW               middleware.Authenticator
	IPGuard              *middleware.IPGuardMiddleware
	IPGuardHandler       *handler.IPGuardHandler
	StaticFS             http.FileSystem
}

// NewRouter 构建应用的 Gin Engine，汇总所有 REST 接口与公共中间件配置。
func NewRouter(opts RouterOptions) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// IP Guard 中间件会在最前层按照 IP 做限流与黑名单处理。
	if opts.IPGuard != nil {
		r.Use(opts.IPGuard.Handle())
	}

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

	if opts.StaticFS != nil {
		r.StaticFS("/static", opts.StaticFS)
	} else {
		r.Static("/static", "./public")
	}

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	api := r.Group("/api")
	{
		if opts.IPGuard != nil && opts.IPGuard.HoneypotPath() != "" {
			// 注册蜜罐接口：正常用户不会请求该路径，如果被访问即视为恶意爬虫。
			// 会把实际的 Gin 路由注册为 /api/<HoneypotPath>，也就是 /api/__internal__/trace。一旦有客户端命中这个接口，中间件就会立刻把该 IP 写入黑名单，后续所有
			// 请求都会直接被 403 拦下，这正是“蜜罐”的作用。
			honeypotPath := strings.TrimLeft(opts.IPGuard.HoneypotPath(), "/")
			if honeypotPath != "" {
				api.Any("/"+honeypotPath, opts.IPGuard.HoneypotHandler())
			}
		}

		authGroup := api.Group("/auth")
		if opts.AuthHandler != nil {
			authGroup.GET("/captcha", opts.AuthHandler.Captcha)
			authGroup.POST("/register", opts.AuthHandler.Register)
			authGroup.POST("/login", opts.AuthHandler.Login)
			authGroup.POST("/refresh", opts.AuthHandler.Refresh)
			authGroup.POST("/logout", opts.AuthHandler.Logout)
			authGroup.POST("/verify-email/request", opts.AuthHandler.RequestEmailVerification)
			authGroup.POST("/verify-email/confirm", opts.AuthHandler.VerifyEmail)
		}

		if opts.AdminMetricsHandler != nil || opts.AdminUserHandler != nil {
			admin := api.Group("/admin")
			if opts.AuthMW != nil {
				admin.Use(opts.AuthMW.Handle())
			}
			if opts.AdminMetricsHandler != nil {
				admin.GET("/metrics", opts.AdminMetricsHandler.Overview)
			}
			if opts.AdminUserHandler != nil {
				admin.GET("/users", opts.AdminUserHandler.Overview)
			}
		}

		// /api/users 下的路由需要登录才能访问，所以单独分组，再挂载 JWT 鉴权中间件。
		userGroup := api.Group("/users")
		if opts.AuthMW != nil {
			userGroup.Use(opts.AuthMW.Handle())
		}
		if opts.UserHandler != nil {
			userGroup.GET("/me", opts.UserHandler.GetMe)
			userGroup.PUT("/me", opts.UserHandler.UpdateMe)
		}

		if opts.ModelHandler != nil {
			models := api.Group("/models")
			if opts.AuthMW != nil {
				models.Use(opts.AuthMW.Handle())
			}
			models.GET("", opts.ModelHandler.List)
			models.POST("", opts.ModelHandler.Create)
			models.POST("/:id/test", opts.ModelHandler.TestConnection)
			models.PUT("/:id", opts.ModelHandler.Update)
			models.DELETE("/:id", opts.ModelHandler.Delete)
		}

		if opts.PromptHandler != nil || opts.PromptCommentHandler != nil {
			prompts := api.Group("/prompts")
			if opts.AuthMW != nil {
				prompts.Use(opts.AuthMW.Handle())
			}
			if opts.PromptHandler != nil {
				prompts.GET("", opts.PromptHandler.ListPrompts)
				prompts.GET("/:id/versions", opts.PromptHandler.ListPromptVersions)
				prompts.GET("/:id/versions/:version", opts.PromptHandler.GetPromptVersion)
				prompts.POST("/export", opts.PromptHandler.ExportPrompts)
				prompts.POST("/import", opts.PromptHandler.ImportPrompts)
				prompts.POST("/:id/share", opts.PromptHandler.SharePrompt)
				prompts.POST("/share/import", opts.PromptHandler.ImportSharedPrompt)
				prompts.POST("/interpret", opts.PromptHandler.Interpret)
				prompts.POST("/ingest", opts.PromptHandler.IngestPrompt)
				prompts.POST("/keywords/augment", opts.PromptHandler.AugmentKeywords)
				prompts.POST("/keywords/manual", opts.PromptHandler.AddManualKeyword)
				prompts.POST("/keywords/remove", opts.PromptHandler.RemoveKeyword)
				prompts.POST("/keywords/sync", opts.PromptHandler.SyncKeywords)
				prompts.POST("/generate", opts.PromptHandler.GeneratePrompt)
				prompts.GET("/:id", opts.PromptHandler.GetPrompt)
				prompts.PATCH("/:id/favorite", opts.PromptHandler.UpdateFavorite)
				prompts.POST("/:id/like", opts.PromptHandler.LikePrompt)
				prompts.DELETE("/:id/like", opts.PromptHandler.UnlikePrompt)
				prompts.DELETE("/:id", opts.PromptHandler.DeletePrompt)
				prompts.POST("", opts.PromptHandler.SavePrompt)
			}
			if opts.PromptCommentHandler != nil {
				prompts.GET("/:id/comments", opts.PromptCommentHandler.List)
				prompts.POST("/:id/comments", opts.PromptCommentHandler.Create)
				prompts.POST("/comments/:id/review", opts.PromptCommentHandler.Review)
				prompts.POST("/comments/:id/like", opts.PromptCommentHandler.Like)
				prompts.DELETE("/comments/:id/like", opts.PromptCommentHandler.Unlike)
				prompts.DELETE("/comments/:id", opts.PromptCommentHandler.Delete)
			}
		}
		if opts.PublicPromptHandler != nil {
			publicPrompts := api.Group("/public-prompts")
			if opts.AuthMW != nil {
				publicPrompts.Use(opts.AuthMW.Handle())
			}
			publicPrompts.GET("", opts.PublicPromptHandler.List)
			publicPrompts.GET("/:id", opts.PublicPromptHandler.Get)
			publicPrompts.POST("/:id/like", opts.PublicPromptHandler.Like)
			publicPrompts.DELETE("/:id/like", opts.PublicPromptHandler.Unlike)
			publicPrompts.POST("/:id/download", opts.PublicPromptHandler.Download)
			publicPrompts.DELETE("/:id", opts.PublicPromptHandler.Delete)
			publicPrompts.POST("", opts.PublicPromptHandler.Submit)
			publicPrompts.POST("/:id/review", opts.PublicPromptHandler.Review)
		}

		if opts.IPGuardHandler != nil {
			ipguard := api.Group("/ip-guard")
			if opts.AuthMW != nil {
				ipguard.Use(opts.AuthMW.Handle())
			}
			// 管理员可在此查看并解除封禁的 IP 黑名单。
			ipguard.GET("/bans", opts.IPGuardHandler.ListBans)
			ipguard.DELETE("/bans/:ip", opts.IPGuardHandler.RemoveBan)
		}

		if opts.UploadHandler != nil {
			uploads := api.Group("/uploads")
			// 头像上传在注册阶段也会使用，所以该路由不强制登录。
			uploads.POST("/avatar", opts.UploadHandler.UploadAvatar)
		}

		if opts.ChangelogHandler != nil {
			logs := api.Group("/changelog")
			logs.GET("", opts.ChangelogHandler.List)

			if opts.AuthMW != nil {
				adminLogs := api.Group("/changelog")
				adminLogs.Use(opts.AuthMW.Handle())
				adminLogs.POST("", opts.ChangelogHandler.Create)
				adminLogs.PUT("/:id", opts.ChangelogHandler.Update)
				adminLogs.DELETE("/:id", opts.ChangelogHandler.Delete)
			}
		}
	}

	return r
}
