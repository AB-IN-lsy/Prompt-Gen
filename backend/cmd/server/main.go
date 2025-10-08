/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 19:55:11
 * @FilePath: \electron-go-app\backend\cmd\server\main.go
 * @LastEditTime: 2025-10-08 19:55:16
 */
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"electron-go-app/backend/internal/app"
	"electron-go-app/backend/internal/handler"
	"electron-go-app/backend/internal/infra/token"
	"electron-go-app/backend/internal/repository"
	"electron-go-app/backend/internal/server"
	authsvc "electron-go-app/backend/internal/service/auth"
)

// main 为服务入口：初始化依赖、启动 HTTP 服务器并处理优雅停机。
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	resources, err := app.Bootstrap(ctx)
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}
	defer func() {
		if err := resources.Close(); err != nil {
			log.Printf("resource cleanup error: %v", err)
		}
	}()

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET not configured")
	}

	accessTTL := parseDurationWithDefault(os.Getenv("JWT_ACCESS_TTL"), 15*time.Minute)
	refreshTTL := parseDurationWithDefault(os.Getenv("JWT_REFRESH_TTL"), 7*24*time.Hour)

	userRepo := repository.NewUserRepository(resources.DBConn())
	jwtManager := token.NewJWTManager(jwtSecret, accessTTL, refreshTTL)
	authService := authsvc.NewService(userRepo, jwtManager)
	authHandler := handler.NewAuthHandler(authService)

	router := server.NewRouter(server.RouterOptions{
		AuthHandler: authHandler,
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: router,
	}

	go func() {
		log.Printf("HTTP server listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
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
