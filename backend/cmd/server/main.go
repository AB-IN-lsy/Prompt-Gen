/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 19:55:11
 * @FilePath: \electron-go-app\backend\cmd\server\main.go
 * @LastEditTime: 2025-10-08 19:55:16
 */
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"electron-go-app/backend/internal/app"
)

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

	log.Printf("nacos endpoint: %s:%d (namespace=%s)", resources.Config.Nacos.Host, resources.Config.Nacos.Port, resources.Config.Nacos.NamespaceID)
	log.Printf("mysql connected: %s@%s/%s", resources.Config.MySQL.Username, resources.Config.MySQL.Host, resources.Config.MySQL.Database)

	<-ctx.Done()
	log.Println("shutdown signal received")
	time.Sleep(500 * time.Millisecond)
}
