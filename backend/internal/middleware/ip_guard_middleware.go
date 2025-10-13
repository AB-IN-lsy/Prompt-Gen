/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-13 23:10:00
 * @FilePath: \electron-go-app\backend\internal\middleware\ip_guard_middleware.go
 * @LastEditTime: 2025-10-13 23:10:00
 */
package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	response "electron-go-app/backend/internal/infra/common"
	appLogger "electron-go-app/backend/internal/infra/logger"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// IPGuardConfig 描述 IP 限流与黑名单的核心参数。
type IPGuardConfig struct {
	Enabled         bool
	Prefix          string
	Window          time.Duration
	MaxRequests     int
	StrikeWindow    time.Duration
	StrikeLimit     int
	BanTTL          time.Duration
	HoneypotPath    string
	AdminScanCount  int
	AdminMaxEntries int
}

// IPGuardMiddleware 基于 Redis 实现 IP 限流与黑名单机制。
type IPGuardMiddleware struct {
	client *redis.Client
	cfg    IPGuardConfig
	logger *zap.SugaredLogger
}

// BlacklistEntry 描述黑名单中的一条记录。
type BlacklistEntry struct {
	IP         string     `json:"ip"`
	TTLSeconds int64      `json:"ttl_seconds"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// NewIPGuardMiddleware 构建 IPGuardMiddleware。
func NewIPGuardMiddleware(client *redis.Client, cfg IPGuardConfig) *IPGuardMiddleware {
	if cfg.Prefix == "" {
		cfg.Prefix = "ipguard"
	}
	if cfg.Window <= 0 {
		cfg.Window = 30 * time.Second
	}
	if cfg.MaxRequests <= 0 {
		cfg.MaxRequests = 120
	}
	if cfg.StrikeWindow <= 0 {
		cfg.StrikeWindow = 10 * time.Minute
	}
	if cfg.StrikeLimit <= 0 {
		cfg.StrikeLimit = 5
	}
	if cfg.BanTTL <= 0 {
		cfg.BanTTL = 30 * time.Minute
	}
	if cfg.HoneypotPath == "" {
		cfg.HoneypotPath = "__internal__/trace"
	}
	if cfg.AdminScanCount <= 0 {
		cfg.AdminScanCount = 100
	}
	if cfg.AdminMaxEntries <= 0 {
		cfg.AdminMaxEntries = 200
	}
	baseLogger := appLogger.S().With("component", "middleware.ipguard")
	return &IPGuardMiddleware{
		client: client,
		cfg:    cfg,
		logger: baseLogger,
	}
}

// HoneypotPath 返回蜜罐接口的访问路径。
func (m *IPGuardMiddleware) HoneypotPath() string {
	return m.cfg.HoneypotPath
}

// Handle 返回 Gin 中间件，实时拦截恶意 IP。
func (m *IPGuardMiddleware) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.cfg.Enabled || m.client == nil {
			c.Next()
			return
		}
		ip := strings.TrimSpace(c.ClientIP())
		if ip == "" {
			c.Next()
			return
		}

		ctx := c.Request.Context()
		if blocked, ttl, err := m.isBlacklisted(ctx, ip); err == nil && blocked {
			m.logger.Infow("blocked by ipguard", "ip", ip, "ttl_seconds", int(ttl.Seconds()))
			response.Fail(c, http.StatusForbidden, response.ErrForbidden, "access temporarily denied", nil)
			c.Abort()
			return
		} else if err != nil {
			m.logger.Warnw("check blacklist failed", "ip", ip, "error", err)
		}

		if allowed, retryAfter, err := m.hit(ctx, ip); err != nil {
			m.logger.Warnw("ip guard allow failed", "ip", ip, "error", err)
			c.Next()
			return
		} else if !allowed {
			if retryAfter > 0 {
				c.Header("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
			}
			response.Fail(c, http.StatusTooManyRequests, response.ErrTooManyRequests, "request rate limited", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// HoneypotHandler 返回蜜罐接口的处理函数，触发后立即拉黑访问者。
func (m *IPGuardMiddleware) HoneypotHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.cfg.Enabled || m.client == nil {
			c.Status(http.StatusNoContent)
			return
		}
		ip := strings.TrimSpace(c.ClientIP())
		if ip == "" {
			c.Status(http.StatusNoContent)
			return
		}
		if err := m.blacklist(c.Request.Context(), ip, m.cfg.BanTTL); err != nil {
			m.logger.Warnw("honeypot blacklist failed", "ip", ip, "error", err)
		} else {
			m.logger.Warnw("honeypot triggered", "ip", ip)
		}
		// 返回 204，不泄露提示，避免普通用户察觉。
		c.Status(http.StatusNoContent)
	}
}

func (m *IPGuardMiddleware) hit(ctx context.Context, ip string) (bool, time.Duration, error) {
	counterKey := fmt.Sprintf("%s:cnt:%s", m.cfg.Prefix, ip)
	pipe := m.client.TxPipeline()
	count := pipe.Incr(ctx, counterKey)
	pipe.Expire(ctx, counterKey, m.cfg.Window)
	if _, err := pipe.Exec(ctx); err != nil {
		return true, 0, err
	}

	if int(count.Val()) <= m.cfg.MaxRequests {
		return true, 0, nil
	}

	retryAfter, err := m.client.TTL(ctx, counterKey).Result()
	if err != nil {
		retryAfter = m.cfg.Window
	}

	if strikeErr := m.recordStrike(ctx, ip); strikeErr != nil {
		m.logger.Warnw("record strike failed", "ip", ip, "error", strikeErr)
	}
	return false, retryAfter, nil
}

func (m *IPGuardMiddleware) recordStrike(ctx context.Context, ip string) error {
	strikeKey := fmt.Sprintf("%s:str:%s", m.cfg.Prefix, ip)
	pipe := m.client.TxPipeline()
	strikes := pipe.Incr(ctx, strikeKey)
	pipe.Expire(ctx, strikeKey, m.cfg.StrikeWindow)
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	if int(strikes.Val()) >= m.cfg.StrikeLimit {
		return m.blacklist(ctx, ip, m.cfg.BanTTL)
	}
	return nil
}

func (m *IPGuardMiddleware) blacklist(ctx context.Context, ip string, ttl time.Duration) error {
	key := fmt.Sprintf("%s:ban:%s", m.cfg.Prefix, ip)
	if err := m.client.Set(ctx, key, "1", ttl).Err(); err != nil {
		return err
	}
	return nil
}

func (m *IPGuardMiddleware) isBlacklisted(ctx context.Context, ip string) (bool, time.Duration, error) {
	key := fmt.Sprintf("%s:ban:%s", m.cfg.Prefix, ip)
	ttl, err := m.client.TTL(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, 0, nil
		}
		return false, 0, err
	}
	if ttl < 0 {
		return false, 0, nil
	}
	return true, ttl, nil
}

// ListBlacklistEntries 汇总 Redis 中仍处于封禁态的 IP，供后台可视化展示。
func (m *IPGuardMiddleware) ListBlacklistEntries(ctx context.Context) ([]BlacklistEntry, error) {
	if m == nil || m.client == nil {
		return nil, fmt.Errorf("ip guard not initialised")
	}

	results := make([]BlacklistEntry, 0)
	keyPattern := fmt.Sprintf("%s:ban:*", m.cfg.Prefix)
	keyPrefix := fmt.Sprintf("%s:ban:", m.cfg.Prefix)
	cursor := uint64(0)
	maxEntries := m.cfg.AdminMaxEntries
	scanCount := m.cfg.AdminScanCount

	for {
		keys, nextCursor, err := m.client.Scan(ctx, cursor, keyPattern, int64(scanCount)).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			ttl, err := m.client.TTL(ctx, key).Result()
			if err != nil {
				if errors.Is(err, redis.Nil) {
					continue
				}
				return nil, err
			}
			if ttl <= 0 {
				continue
			}

			ip := strings.TrimPrefix(key, keyPrefix)
			ttlSeconds := ttl / time.Second
			if ttl%time.Second != 0 {
				ttlSeconds++
			}
			entry := BlacklistEntry{
				IP:         ip,
				TTLSeconds: int64(ttlSeconds),
			}
			expiresAt := time.Now().Add(ttl).UTC()
			entry.ExpiresAt = &expiresAt

			results = append(results, entry)
			if maxEntries > 0 && len(results) >= maxEntries {
				return results, nil
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
		if maxEntries > 0 && len(results) >= maxEntries {
			break
		}
	}

	return results, nil
}

// RemoveFromBlacklist 允许管理员手动解除某个 IP 的封禁。
func (m *IPGuardMiddleware) RemoveFromBlacklist(ctx context.Context, ip string) error {
	if m == nil || m.client == nil {
		return fmt.Errorf("ip guard not initialised")
	}

	cleanIP := strings.TrimSpace(ip)
	if cleanIP == "" {
		return fmt.Errorf("ip required")
	}

	key := fmt.Sprintf("%s:ban:%s", m.cfg.Prefix, cleanIP)
	if err := m.client.Del(ctx, key).Err(); err != nil {
		return err
	}
	return nil
}
