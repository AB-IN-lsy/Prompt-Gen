package server

import (
	"fmt"
	"time"

	"electron-go-app/backend/internal/handler"

	"github.com/gin-gonic/gin"
)

type RouterOptions struct {
	AuthHandler *handler.AuthHandler
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
			authGroup.POST("/register", opts.AuthHandler.Register)
			authGroup.POST("/login", opts.AuthHandler.Login)
		}
	}

	return r
}
