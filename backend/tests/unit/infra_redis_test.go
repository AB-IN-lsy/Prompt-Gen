/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 16:40:11
 * @FilePath: \electron-go-app\backend\tests\unit\infra_redis_test.go
 * @LastEditTime: 2025-10-09 16:40:12
 */
package unit

import (
	"context"
	"strconv"
	"testing"
	"time"

	"electron-go-app/backend/internal/config"
	"electron-go-app/backend/internal/infra"

	miniredis "github.com/alicebob/miniredis/v2"
)

func TestNewDefaultRedisOptions_FromEnv(t *testing.T) {
	config.SetEnvFileLoadingForTest(false)
	t.Cleanup(func() { config.SetEnvFileLoadingForTest(true) })

	t.Setenv("REDIS_ENDPOINT", "127.0.0.1:6380")
	t.Setenv("REDIS_PASSWORD", "secret")
	t.Setenv("REDIS_DB", "2")

	opts, err := infra.NewDefaultRedisOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.Host != "127.0.0.1" || opts.Port != 6380 {
		t.Fatalf("unexpected host/port: %s:%d", opts.Host, opts.Port)
	}
	if opts.Password != "secret" {
		t.Fatalf("expected password to match")
	}
	if opts.DB != 2 {
		t.Fatalf("expected db=2, got %d", opts.DB)
	}
}

func TestNewDefaultRedisOptions_Defaults(t *testing.T) {
	config.SetEnvFileLoadingForTest(false)
	t.Cleanup(func() { config.SetEnvFileLoadingForTest(true) })

	t.Setenv("REDIS_ENDPOINT", "10.0.0.2")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")

	opts, err := infra.NewDefaultRedisOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.Host != "10.0.0.2" {
		t.Fatalf("expected host 10.0.0.2, got %s", opts.Host)
	}
	if opts.Port != 6379 {
		t.Fatalf("expected default port 6379, got %d", opts.Port)
	}
	if opts.DB != 0 {
		t.Fatalf("expected default db 0, got %d", opts.DB)
	}
}

func TestNewDefaultRedisOptions_MissingEndpoint(t *testing.T) {
	config.SetEnvFileLoadingForTest(false)
	t.Cleanup(func() { config.SetEnvFileLoadingForTest(true) })

	if _, err := infra.NewDefaultRedisOptions(); err == nil {
		t.Fatalf("expected error when endpoint missing")
	}
}

func TestNewRedisClient(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer server.Close()

	port, err := strconv.Atoi(server.Port())
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	opts := infra.RedisOptions{
		Host:    server.Host(),
		Port:    port,
		Timeout: time.Second,
	}

	client, err := infra.NewRedisClient(opts)
	if err != nil {
		t.Fatalf("NewRedisClient returned error: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	if err := client.Set(ctx, "foo", "bar", 0).Err(); err != nil {
		t.Fatalf("redis set: %v", err)
	}

	val, err := client.Get(ctx, "foo").Result()
	if err != nil {
		t.Fatalf("redis get: %v", err)
	}
	if val != "bar" {
		t.Fatalf("expected value bar, got %s", val)
	}
}
