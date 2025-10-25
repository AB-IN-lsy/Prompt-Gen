package unit

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"
	"time"

	"electron-go-app/backend/internal/infra/captcha"
	"electron-go-app/backend/internal/infra/ratelimit"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newRedisClient(t *testing.T) (*redis.Client, func()) {
	t.Helper()

	server, err := miniredis.Run()
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) && (errors.Is(opErr.Err, syscall.EPERM) || errors.Is(opErr.Err, syscall.EACCES)) {
			t.Skipf("当前环境禁止监听端口: %v", err)
		}
		t.Fatalf("start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{Addr: server.Addr()})

	cleanup := func() {
		_ = client.Close()
		server.Close()
	}

	return client, cleanup
}

func TestManagerGenerateAndVerify(t *testing.T) {
	client, cleanup := newRedisClient(t)
	defer cleanup()

	limiter := ratelimit.NewRedisLimiter(client, "captcha_test")
	manager := captcha.NewManager(client, limiter, captcha.Options{
		Prefix:          "test-captcha",
		TTL:             time.Minute,
		Length:          4,
		RateLimitPerMin: 5,
	})

	ctx := context.Background()
	id, _, remaining, err := manager.Generate(ctx, "127.0.0.1")
	if err != nil {
		t.Fatalf("generate captcha: %v", err)
	}
	if id == "" {
		t.Fatalf("expected id to be non-empty")
	}
	if remaining != 4 {
		t.Fatalf("expected remaining attempts to be 4, got %d", remaining)
	}

	stored, err := client.Get(ctx, "test-captcha:"+id).Result()
	if err != nil {
		t.Fatalf("get stored answer: %v", err)
	}

	if err := manager.Verify(ctx, id, stored); err != nil {
		t.Fatalf("verify captcha: %v", err)
	}

	if _, err := client.Get(ctx, "test-captcha:"+id).Result(); err == nil {
		t.Fatalf("expected captcha entry to be deleted after verify")
	}
}

func TestManagerVerifyMismatch(t *testing.T) {
	client, cleanup := newRedisClient(t)
	defer cleanup()

	limiter := ratelimit.NewRedisLimiter(client, "captcha_test")
	manager := captcha.NewManager(client, limiter, captcha.Options{Prefix: "c", TTL: time.Minute, RateLimitPerMin: 5})
	ctx := context.Background()

	id, _, _, err := manager.Generate(ctx, "10.0.0.2")
	if err != nil {
		t.Fatalf("generate captcha: %v", err)
	}

	if err := manager.Verify(ctx, id, "wrong"); err != captcha.ErrCaptchaMismatch {
		t.Fatalf("expected mismatch error, got %v", err)
	}
}

func TestManagerVerifyMissing(t *testing.T) {
	client, cleanup := newRedisClient(t)
	defer cleanup()

	limiter := ratelimit.NewRedisLimiter(client, "captcha_test")
	manager := captcha.NewManager(client, limiter, captcha.Options{Prefix: "c", TTL: time.Minute, RateLimitPerMin: 5})
	ctx := context.Background()

	id, _, _, err := manager.Generate(ctx, "10.0.0.3")
	if err != nil {
		t.Fatalf("generate captcha: %v", err)
	}

	if err := client.Del(ctx, "c:"+id).Err(); err != nil {
		t.Fatalf("delete entry: %v", err)
	}

	if err := manager.Verify(ctx, id, "whatever"); err != captcha.ErrCaptchaNotFound {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestManagerRateLimit(t *testing.T) {
	client, cleanup := newRedisClient(t)
	defer cleanup()

	limiter := ratelimit.NewRedisLimiter(client, "captcha_test")
	manager := captcha.NewManager(client, limiter, captcha.Options{
		Prefix:          "rl",
		TTL:             time.Minute,
		RateLimitPerMin: 1,
	})

	ctx := context.Background()
	if _, _, remaining, err := manager.Generate(ctx, "8.8.8.8"); err != nil {
		t.Fatalf("first generate should succeed: %v", err)
	} else if remaining != 0 {
		t.Fatalf("expected remaining attempts to be 0 after first request, got %d", remaining)
	}

	if _, _, _, err := manager.Generate(ctx, "8.8.8.8"); err != captcha.ErrRateLimited {
		t.Fatalf("expected rate limited error, got %v", err)
	}
}
