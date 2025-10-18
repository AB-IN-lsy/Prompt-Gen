/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 19:55:11
 * @FilePath: \electron-go-app\backend\cmd\server\main.go
 * @LastEditTime: 2025-10-09 20:42:05
 */
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"electron-go-app/backend/internal/app"
	"electron-go-app/backend/internal/bootstrap"
	"electron-go-app/backend/internal/config"
	"electron-go-app/backend/internal/infra/logger"

	"go.uber.org/zap"
)

// main 为服务入口：初始化依赖、启动 HTTP 服务器并处理优雅停机。
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := initLogger()
	defer logger.Sync()
	sugar := logger.Sugar()

	resources := mustBootstrap(ctx, sugar)
	defer closeResources(resources, sugar)

	runtimeCfg := loadRuntimeConfig(resources, sugar)
	app, err := bootstrap.BuildApplication(ctx, sugar, resources, bootstrap.RuntimeConfig{
		Port:       runtimeCfg.port,
		JWTSecret:  runtimeCfg.jwtSecret,
		AccessTTL:  runtimeCfg.accessTTL,
		RefreshTTL: runtimeCfg.refreshTTL,
		Mode:       runtimeCfg.mode,
		LocalUser:  runtimeCfg.local,
	})
	if err != nil {
		sugar.Fatalw("build application failed", "error", err)
	}

	server := &http.Server{
		Addr:    ":" + runtimeCfg.port,
		Handler: app.Router,
	}

	go func() {
		sugar.Infow("http server listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			sugar.Fatalw("server listen failed", "error", err)
		}
	}()

	<-ctx.Done()
	sugar.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		sugar.Errorw("server shutdown error", "error", err)
	}
}

func initLogger() *zap.Logger {
	zapLogger, err := logger.Init()
	if err != nil {
		panic(err)
	}
	return zapLogger
}

func mustBootstrap(ctx context.Context, sugar *zap.SugaredLogger) *app.Resources {
	resources, err := app.InitResources(ctx)
	if err != nil {
		sugar.Fatalw("bootstrap failed", "error", err)
	}
	return resources
}

func closeResources(resources *app.Resources, sugar *zap.SugaredLogger) {
	if err := resources.Close(); err != nil {
		sugar.Warnw("resource cleanup error", "error", err)
	}
}

type runtimeConfig struct {
	port       string
	jwtSecret  string
	accessTTL  time.Duration
	refreshTTL time.Duration
	mode       string
	local      config.LocalRuntime
}

func loadRuntimeConfig(resources *app.Resources, sugar *zap.SugaredLogger) runtimeConfig {
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "9090"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	mode := resources.Config.Mode
	if jwtSecret == "" && !strings.EqualFold(mode, config.ModeLocal) {
		sugar.Fatal("JWT_SECRET not configured")
	}
	if jwtSecret == "" {
		jwtSecret = "local-mode-secret"
	}

	accessTTL := parseDurationWithDefault(os.Getenv("JWT_ACCESS_TTL"), 15*time.Minute)
	refreshTTL := parseDurationWithDefault(os.Getenv("JWT_REFRESH_TTL"), 7*24*time.Hour)

	return runtimeConfig{
		port:       port,
		jwtSecret:  jwtSecret,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		mode:       mode,
		local:      resources.Config.Local,
	}
}

// parseDurationWithDefault 解析时长字符串，失败时返回预设的回退值。
func parseDurationWithDefault(value string, fallback time.Duration) time.Duration {
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return d
}
