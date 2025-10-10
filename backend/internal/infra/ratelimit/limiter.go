/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 17:01:17
 * @FilePath: \electron-go-app\backend\internal\infra\ratelimit\limiter.go
 * @LastEditTime: 2025-10-10 17:01:21
 */
package ratelimit

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// AllowResult 描述限流请求的结果。
type AllowResult struct {
	Allowed    bool
	RetryAfter time.Duration
	Remaining  int
}

// Limiter 定义限流器的通用能力。
type Limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (AllowResult, error)
}

// RedisLimiter 使用 Redis 实现简单的计数限流。
type RedisLimiter struct {
	client *redis.Client
	prefix string
}

func NewRedisLimiter(client *redis.Client, prefix string) *RedisLimiter {
	if prefix == "" {
		prefix = "ratelimit"
	}
	return &RedisLimiter{client: client, prefix: prefix}
}

func (r *RedisLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (AllowResult, error) {
	if limit <= 0 {
		return AllowResult{Allowed: true, Remaining: -1}, nil
	}
	if window <= 0 {
		window = time.Minute
	}
	if r == nil || r.client == nil {
		return AllowResult{Allowed: true, Remaining: -1}, nil
	}

	namespaced := r.prefix + ":" + key
	pipe := r.client.TxPipeline()
	counter := pipe.Incr(ctx, namespaced)
	pipe.Expire(ctx, namespaced, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return AllowResult{}, err
	}

	count := int(counter.Val())
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}

	if count > limit {
		ttl, err := r.client.TTL(ctx, namespaced).Result()
		if err != nil {
			return AllowResult{}, err
		}
		if ttl < 0 {
			ttl = window
		}
		return AllowResult{Allowed: false, RetryAfter: ttl, Remaining: 0}, nil
	}

	return AllowResult{Allowed: true, Remaining: remaining}, nil
}

// MemoryLimiter 是 Redis 不可用时的替代方案，仅用于开发环境。
type MemoryLimiter struct {
	mu    sync.Mutex
	store map[string]entry
}

type entry struct {
	count   int
	expires time.Time
}

func NewMemoryLimiter() *MemoryLimiter {
	return &MemoryLimiter{store: make(map[string]entry)}
}

func (m *MemoryLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (AllowResult, error) {
	if limit <= 0 {
		return AllowResult{Allowed: true, Remaining: -1}, nil
	}
	if window <= 0 {
		window = time.Minute
	}
	if m == nil {
		return AllowResult{Allowed: true, Remaining: -1}, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	ent, ok := m.store[key]
	if !ok || now.After(ent.expires) {
		m.store[key] = entry{count: 1, expires: now.Add(window)}
		return AllowResult{Allowed: true, Remaining: limit - 1}, nil
	}

	ent.count++
	m.store[key] = entry{count: ent.count, expires: ent.expires}

	remaining := limit - ent.count
	if remaining < 0 {
		remaining = 0
	}

	if ent.count > limit {
		return AllowResult{Allowed: false, RetryAfter: time.Until(ent.expires), Remaining: 0}, nil
	}

	return AllowResult{Allowed: true, Remaining: remaining}, nil
}
