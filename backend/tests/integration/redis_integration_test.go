//go:build integration

/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 16:43:57
 * @FilePath: \electron-go-app\backend\tests\integration\redis_integration_test.go
 * @LastEditTime: 2025-10-09 16:44:01
 */

package integration

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	infra "electron-go-app/backend/internal/infra/client"
)

func TestRedisConnection(t *testing.T) {
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("integration tests disabled")
	}

	if os.Getenv("REDIS_ENDPOINT") == "" {
		t.Skip("REDIS_ENDPOINT not set")
	}

	opts, err := infra.NewDefaultRedisOptions()
	if err != nil {
		t.Fatalf("failed to build redis options: %v", err)
	}

	client, err := infra.NewRedisClient(opts)
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	key := fmt.Sprintf("integration:redis:%d", time.Now().UnixNano())
	expected := "ok"

	if err := client.Set(ctx, key, expected, time.Minute).Err(); err != nil {
		t.Fatalf("redis set failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Del(context.Background(), key).Err() })

	val, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("redis get failed: %v", err)
	}
	if val != expected {
		t.Fatalf("unexpected value, want %s, got %s", expected, val)
	}

	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("redis ttl failed: %v", err)
	}
	if ttl <= 0 {
		t.Fatalf("expected ttl to be positive, got %v", ttl)
	}

	// 允许通过 REDIS_EXPECT_DB 检查是否连接到预期的 DB。
	if rawDB := os.Getenv("REDIS_EXPECT_DB"); rawDB != "" {
		expectedDB, convErr := strconv.Atoi(rawDB)
		if convErr != nil {
			t.Fatalf("invalid REDIS_EXPECT_DB: %v", convErr)
		}
		if int(client.Options().DB) != expectedDB {
			t.Fatalf("connected to unexpected db, want %d, got %d", expectedDB, client.Options().DB)
		}
	}
}
