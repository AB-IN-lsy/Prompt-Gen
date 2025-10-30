/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 17:01:17
 * @FilePath: \electron-go-app\backend\internal\infra\ratelimit\limiter.go
 * @LastEditTime: 2025-10-10 17:01:21
 */
package ratelimit

import (
	"context"
	"errors"
	"strconv"
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

// NewRedisLimiter 根据 Redis 客户端构造限流器，可自定义 key 前缀。
func NewRedisLimiter(client *redis.Client, prefix string) *RedisLimiter {
	if prefix == "" {
		prefix = "ratelimit"
	}
	return &RedisLimiter{client: client, prefix: prefix}
}

// Allow 以 Redis 计数器实现固定窗口限流，返回是否放行、剩余次数与等待时间。
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

// Peek 返回指定 key 当前的计数与剩余有效期。
func (r *RedisLimiter) Peek(ctx context.Context, key string) (int, time.Duration, error) {
	if r == nil || r.client == nil {
		return 0, 0, nil
	}
	namespaced := r.prefix + ":" + key
	value, err := r.client.Get(ctx, namespaced).Result()
	if errors.Is(err, redis.Nil) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	count, err := strconv.Atoi(value)
	if err != nil {
		return 0, 0, err
	}
	ttl, err := r.client.TTL(ctx, namespaced).Result()
	if err != nil {
		return 0, 0, err
	}
	return count, ttl, nil
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

// NewMemoryLimiter 构建内存版限流器，常用于本地开发与单元测试。
func NewMemoryLimiter() *MemoryLimiter {
	return &MemoryLimiter{store: make(map[string]entry)}
}

// Allow 通过内存 map 统计请求次数，模拟 Redis 的固定窗口限流行为。
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

// Peek 获取内存限流器中指定 key 的计数与剩余有效期。
func (m *MemoryLimiter) Peek(key string) (int, time.Duration) {
	if m == nil {
		return 0, 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ent, ok := m.store[key]
	if !ok {
		return 0, 0
	}
	remaining := time.Until(ent.expires)
	if remaining < 0 {
		delete(m.store, key)
		return 0, 0
	}
	return ent.count, remaining
}
