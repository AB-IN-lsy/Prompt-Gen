/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 21:25:41
 * @FilePath: \electron-go-app\backend\internal\infra\token\refresh_store.go
 * @LastEditTime: 2025-10-09 21:25:47
 */
package token

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultRefreshPrefix = "auth:refresh"

// RedisRefreshTokenStore 使用 Redis 保存刷新令牌，支持分布式实例共享状态。
//
// 刷新流程概念小结：
//  1. 用户登录后拿到 access token + refresh token，其中 refresh token 会带一个唯一的 jti。
//  2. 后端在这里把 <userID, jti> 落到 Redis，并设定过期时间 —— 这跟 refresh token 的 exp 恒保持一致。
//  3. 当 access token 过期时，客户端调用 /api/auth/refresh，并带上 refresh token。
//     3.1 服务端解析 refresh token，获取 jti -> 调 Exists() 判断它还在不在。
//     3.2 如果在，就 Delete() 掉旧的记录，重新签发一对新的 access/refresh，并调用 Save() 写入新的 jti。
//  4. 当用户主动登出，或 refresh token 被服务器判定失效时，调用 Delete() 移除记录。
//  5. 如果 refresh token 过期（到了 exp/TTL），记录会自然失效，客户端只能重新登录。
//
// 因为 Redis 是外部存储，这些状态跨实例共享，也能抵御服务重启导致的“全部续命”风险。
type RedisRefreshTokenStore struct {
	client *redis.Client
	prefix string
}

// NewRedisRefreshTokenStore 构造 Redis 刷新令牌存储。
func NewRedisRefreshTokenStore(client *redis.Client, prefix string) *RedisRefreshTokenStore {
	if prefix == "" {
		prefix = defaultRefreshPrefix
	}
	return &RedisRefreshTokenStore{client: client, prefix: prefix}
}

func (s *RedisRefreshTokenStore) key(userID uint, tokenID string) string {
	return fmt.Sprintf("%s:%d:%s", s.prefix, userID, tokenID)
}

// Save 将刷新令牌存入 Redis，并设置过期时间。
// - userID/tokenID 对应用户与刷新令牌的唯一指纹。
// - expiresAt 通常传入 refresh token 的 exp，保证 Redis 记录“过期即失效”。
// 若计算出的 TTL 小于等于 0，说明 token 已经过期，仍然保存意义不大，这里回退到 1s 以确保键马上失效。
func (s *RedisRefreshTokenStore) Save(ctx context.Context, userID uint, tokenID string, expiresAt time.Time) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("redis client not configured")
	}
	if tokenID == "" {
		return fmt.Errorf("token id required")
	}

	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		ttl = time.Second
	}

	return s.client.Set(ctx, s.key(userID, tokenID), "1", ttl).Err()
}

// Delete 从 Redis 中移除刷新令牌。
// 常见调用场景：刷新流程中“先删旧、再写新”以及用户主动登出。
func (s *RedisRefreshTokenStore) Delete(ctx context.Context, userID uint, tokenID string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("redis client not configured")
	}
	if tokenID == "" {
		return nil
	}
	return s.client.Del(ctx, s.key(userID, tokenID)).Err()
}

// Exists 检查刷新令牌是否仍有效。
// 如果 Redis 返回 0，说明该 jti 已经被删除或超时 —— 这会被业务层视为“刷新令牌失效/被吊销”。
func (s *RedisRefreshTokenStore) Exists(ctx context.Context, userID uint, tokenID string) (bool, error) {
	if s == nil || s.client == nil {
		return false, fmt.Errorf("redis client not configured")
	}
	if tokenID == "" {
		return false, nil
	}
	count, err := s.client.Exists(ctx, s.key(userID, tokenID)).Result()
	if err != nil {
		return false, err
	}
	return count == 1, nil
}

// MemoryRefreshTokenStore 便于测试以及无 Redis 环境下的退化处理（进程内有效）。
// 它只在当前进程内生效：一旦服务重启，之前的刷新令牌都会失效，需要用户重新登录。
type MemoryRefreshTokenStore struct {
	mu     sync.RWMutex
	tokens map[uint]map[string]time.Time
}

// NewMemoryRefreshTokenStore 创建进程内刷新令牌存储。
func NewMemoryRefreshTokenStore() *MemoryRefreshTokenStore {
	return &MemoryRefreshTokenStore{tokens: make(map[uint]map[string]time.Time)}
}

// Save 存储刷新令牌。
// 结构与 Redis 版本一致：Map 的 key 是 userID，再内嵌一个 tokenID -> expiresAt 的 Map。
func (s *MemoryRefreshTokenStore) Save(_ context.Context, userID uint, tokenID string, expiresAt time.Time) error {
	if tokenID == "" {
		return fmt.Errorf("token id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tokens[userID]; !ok {
		s.tokens[userID] = make(map[string]time.Time)
	}
	s.tokens[userID][tokenID] = expiresAt
	return nil
}

// Delete 移除刷新令牌。
// 如果该用户名下已经没有其它刷新令牌，会顺便把用户那一层 map 删除，避免空 map 占位。
func (s *MemoryRefreshTokenStore) Delete(_ context.Context, userID uint, tokenID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if bucket, ok := s.tokens[userID]; ok {
		delete(bucket, tokenID)
		if len(bucket) == 0 {
			delete(s.tokens, userID)
		}
	}
	return nil
}

// Exists 检测令牌是否存在且未过期。
// 额外逻辑：访问时如果发现已经过期，会顺带触发 Delete 来清理陈旧数据，避免内存堆积。
func (s *MemoryRefreshTokenStore) Exists(_ context.Context, userID uint, tokenID string) (bool, error) {
	s.mu.RLock()
	bucket, ok := s.tokens[userID]
	if !ok {
		s.mu.RUnlock()
		return false, nil
	}
	expiresAt, ok := bucket[tokenID]
	if !ok {
		s.mu.RUnlock()
		return false, nil
	}
	if time.Now().After(expiresAt) {
		s.mu.RUnlock()
		// 清理过期条目，避免内存堆积。
		s.Delete(context.Background(), userID, tokenID)
		return false, nil
	}
	s.mu.RUnlock()
	return true, nil
}
