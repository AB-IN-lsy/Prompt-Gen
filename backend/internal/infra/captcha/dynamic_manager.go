package captcha

import (
	"context"
	"errors"
	"sync/atomic"

	"electron-go-app/backend/internal/infra/ratelimit"

	"github.com/redis/go-redis/v9"
)

// DynamicManager 包装底层 Manager，实现热更新能力。
type DynamicManager struct {
	redis   *redis.Client
	limiter ratelimit.Limiter

	current atomic.Value // *Manager
	enabled atomic.Bool
	options atomic.Value // Options
}

// NewDynamicManager 创建动态管理器包装器。
func NewDynamicManager(redisClient *redis.Client, limiter ratelimit.Limiter) *DynamicManager {
	if redisClient == nil {
		panic("dynamic captcha manager requires redis client")
	}
	if limiter == nil {
		limiter = ratelimit.NewMemoryLimiter()
	}
	d := &DynamicManager{
		redis:   redisClient,
		limiter: limiter,
	}
	d.current.Store((*Manager)(nil))
	d.enabled.Store(false)
	d.options.Store(Options{})
	return d
}

// Swap 根据最新配置替换底层 Manager。
func (d *DynamicManager) Swap(opts Options, enabled bool) error {
	if !enabled {
		d.current.Store((*Manager)(nil))
		d.enabled.Store(false)
		d.options.Store(Options{})
		return nil
	}

	manager := NewManager(d.redis, d.limiter, opts)
	d.current.Store(manager)
	d.enabled.Store(true)
	d.options.Store(opts)
	return nil
}

// Enabled 返回当前是否启用了验证码管理器。
func (d *DynamicManager) Enabled() bool {
	return d != nil && d.enabled.Load()
}

func (d *DynamicManager) getManager() (*Manager, error) {
	if !d.Enabled() {
		return nil, ErrCaptchaDisabled
	}
	raw := d.current.Load()
	manager, _ := raw.(*Manager)
	if manager == nil {
		return nil, ErrCaptchaDisabled
	}
	return manager, nil
}

// Generate 实现 CaptchaManager 接口，调用当前启用的 Manager。
func (d *DynamicManager) Generate(ctx context.Context, ip string) (string, string, int, error) {
	manager, err := d.getManager()
	if err != nil {
		return "", "", 0, err
	}
	return manager.Generate(ctx, ip)
}

// Verify 实现 CaptchaManager 接口，调用当前启用的 Manager。
func (d *DynamicManager) Verify(ctx context.Context, id string, answer string) error {
	manager, err := d.getManager()
	if err != nil {
		return err
	}
	return manager.Verify(ctx, id, answer)
}

// ManagerOrNil 返回当前底层 Manager，便于调试或测试。
func (d *DynamicManager) ManagerOrNil() (*Manager, error) {
	manager, err := d.getManager()
	if errors.Is(err, ErrCaptchaDisabled) {
		return nil, nil
	}
	return manager, err
}

// Options 返回当前加载的验证码配置。
func (d *DynamicManager) Options() Options {
	if d == nil {
		return Options{}
	}
	if raw := d.options.Load(); raw != nil {
		if opts, ok := raw.(Options); ok {
			return opts
		}
	}
	return Options{}
}
