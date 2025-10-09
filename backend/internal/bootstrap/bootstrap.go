/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 20:51:28
 * @FilePath: \electron-go-app\backend\internal\bootstrap\bootstrap.go
 * @LastEditTime: 2025-10-09 20:51:34
 */
package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"electron-go-app/backend/internal/app"
	"electron-go-app/backend/internal/handler"
	"electron-go-app/backend/internal/infra/captcha"
	"electron-go-app/backend/internal/infra/token"
	"electron-go-app/backend/internal/middleware"
	"electron-go-app/backend/internal/repository"
	"electron-go-app/backend/internal/server"
	authsvc "electron-go-app/backend/internal/service/auth"
	usersvc "electron-go-app/backend/internal/service/user"

	"go.uber.org/zap"
)

type RuntimeConfig struct {
	Port       string
	JWTSecret  string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type Application struct {
	Resources *app.Resources
	AuthSvc   *authsvc.Service
	UserSvc   *usersvc.Service
	Router    http.Handler
}

func BuildApplication(ctx context.Context, logger *zap.SugaredLogger, resources *app.Resources, cfg RuntimeConfig) (*Application, error) {
	userRepo := repository.NewUserRepository(resources.DBConn())
	tokens := token.NewJWTManager(cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL)

	var refreshStore authsvc.RefreshTokenStore
	if resources.Redis != nil {
		refreshStore = token.NewRedisRefreshTokenStore(resources.Redis, "")
	} else {
		refreshStore = token.NewMemoryRefreshTokenStore()
		logger.Infow("using in-memory refresh token store; tokens won't persist across restarts")
	}

	captchaManager, err := initCaptchaManager(resources, logger)
	if err != nil {
		return nil, err
	}

	authService := authsvc.NewService(userRepo, tokens, refreshStore, captchaManager)
	authHandler := handler.NewAuthHandler(authService)

	userService := usersvc.NewService(userRepo)
	userHandler := handler.NewUserHandler(userService)

	uploadStorage := filepath.Join("public", "avatars")
	uploadHandler := handler.NewUploadHandler(uploadStorage)

	authMiddleware := middleware.NewAuthMiddleware(cfg.JWTSecret)

	router := server.NewRouter(server.RouterOptions{
		AuthHandler:   authHandler,
		UserHandler:   userHandler,
		UploadHandler: uploadHandler,
		AuthMW:        authMiddleware,
	})

	return &Application{
		Resources: resources,
		AuthSvc:   authService,
		UserSvc:   userService,
		Router:    router,
	}, nil
}

func initCaptchaManager(resources *app.Resources, logger *zap.SugaredLogger) (authsvc.CaptchaManager, error) {
	captchaOpts, captchaEnabled, err := captcha.LoadOptionsFromEnv()
	if err != nil {
		logger.Errorw("load captcha config failed", "error", err)
		return nil, fmt.Errorf("load captcha config: %w", err)
	}

	if !captchaEnabled {
		return nil, nil
	}

	if resources.Redis == nil {
		return nil, fmt.Errorf("captcha enabled but redis not configured")
	}

	manager := captcha.NewManager(resources.Redis, captchaOpts)
	logger.Infow("captcha enabled", "prefix", captchaOpts.Prefix, "ttl", captchaOpts.TTL)
	return manager, nil
}
