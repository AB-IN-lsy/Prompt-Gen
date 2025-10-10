package captcha

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"electron-go-app/backend/internal/infra/ratelimit"

	"github.com/mojocn/base64Captcha"
	"github.com/redis/go-redis/v9"
)

var (
	ErrCaptchaNotFound = errors.New("captcha not found or expired")
	ErrCaptchaMismatch = errors.New("captcha code mismatch")
	ErrRateLimited     = errors.New("captcha requests too frequent")
)

type Generator interface {
	Generate(ctx context.Context, ip string) (id string, b64 string, remaining int, err error)
}

type Verifier interface {
	Verify(ctx context.Context, id string, answer string) error
}

// Manager 封装验证码生成、答案存储以及按 IP 限流的完整逻辑。
type Manager struct {
	store   *redis.Client        // Redis 客户端，负责缓存验证码答案
	driver  base64Captcha.Driver // 负责生成具体的验证码图片与答案
	prefix  string               // Redis Key 前缀，避免不同业务污染
	ttl     time.Duration        // 验证码存活时间
	limiter ratelimit.Limiter    // 可选限流器，实现统一的限流策略
	limit   int                  // 指定窗口内允许的最大请求次数
	window  time.Duration        // 限流窗口长度
}

// Options 聚合了验证码图像参数以及限流设置，可通过环境变量动态配置。
type Options struct {
	Prefix          string
	TTL             time.Duration
	Width           int
	Height          int
	Length          int
	MaxSkew         float64
	DotCount        int
	RateLimitPerMin int
	// RateLimitWindow 控制单个 IP 的计数窗口长度，超过该时间自动清零。
	RateLimitWindow time.Duration
}

const (
	defaultPrefix  = "captcha"       // 默认 Redis Key 前缀
	defaultTTL     = 5 * time.Minute // 验证码默认过期时间
	defaultWidth   = 240             // 默认图片宽度
	defaultHeight  = 80              // 默认图片高度
	defaultLength  = 5               // 默认验证码位数
	defaultMaxSkew = 0.7             // 默认字符扭曲程度
	defaultDot     = 80              // 默认噪点数量
)

// NewManager 根据给定的选项构造验证码管理器，实现生成、校验与限流。
func NewManager(redisClient *redis.Client, limiter ratelimit.Limiter, opts Options) *Manager {
	if redisClient == nil {
		panic("captcha manager requires redis client")
	}

	prefix := opts.Prefix
	if strings.TrimSpace(prefix) == "" {
		prefix = defaultPrefix
	}

	ttl := opts.TTL
	if ttl <= 0 {
		ttl = defaultTTL
	}

	width := opts.Width
	if width <= 0 {
		width = defaultWidth
	}

	height := opts.Height
	if height <= 0 {
		height = defaultHeight
	}

	length := opts.Length
	if length <= 0 {
		length = defaultLength
	}

	maxSkew := opts.MaxSkew
	if maxSkew <= 0 {
		maxSkew = defaultMaxSkew
	}

	dotCount := opts.DotCount
	if dotCount <= 0 {
		dotCount = defaultDot
	}

	// 目前采用纯数字验证码方案，后续如需引入复杂图形可以替换 Driver。
	driver := base64Captcha.NewDriverDigit(height, width, length, maxSkew, dotCount)

	maxHits := opts.RateLimitPerMin
	if maxHits < 0 {
		maxHits = 0
	}

	rlTTL := opts.RateLimitWindow
	if rlTTL <= 0 {
		rlTTL = time.Minute
	}

	if limiter == nil {
		limiter = ratelimit.NewMemoryLimiter()
	}

	return &Manager{
		store:   redisClient,
		driver:  driver,
		prefix:  prefix,
		ttl:     ttl,
		limiter: limiter,
		limit:   maxHits,
		window:  rlTTL,
	}
}

// Generate 输出 base64 图像和对应的验证码 ID，并在 Redis 中缓存答案。
func (m *Manager) Generate(ctx context.Context, ip string) (string, string, int, error) {
	// 先做简单的 IP 限流，防止爬虫无限制刷验证码，同时记录剩余额度。
	remaining, err := m.checkRateLimit(ctx, ip)
	if err != nil {
		return "", "", 0, err
	}

	id, content, answer := m.driver.GenerateIdQuestionAnswer()

	item, err := m.driver.DrawCaptcha(content)
	if err != nil {
		return "", "", 0, fmt.Errorf("draw captcha: %w", err)
	}

	b64 := item.EncodeB64string()

	if err := m.store.Set(ctx, m.key(id), strings.ToLower(answer), m.ttl).Err(); err != nil {
		return "", "", 0, fmt.Errorf("store captcha: %w", err)
	}

	return id, b64, remaining, nil
}

// Verify 对比用户提交的验证码答案，成功时删除缓存，失败时返回明确错误。
func (m *Manager) Verify(ctx context.Context, id string, answer string) error {
	if strings.TrimSpace(id) == "" {
		return ErrCaptchaNotFound
	}

	key := m.key(id)
	stored, err := m.store.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrCaptchaNotFound
		}
		return fmt.Errorf("get captcha: %w", err)
	}

	if err := m.store.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("delete captcha: %w", err)
	}

	if !strings.EqualFold(strings.TrimSpace(answer), stored) {
		return ErrCaptchaMismatch
	}

	return nil
}

// key 统一生成 Redis Key，减少散落的格式字符串。
func (m *Manager) key(id string) string {
	return fmt.Sprintf("%s:%s", m.prefix, id)
}

// checkRateLimit 维护单个 IP 的访问频次，超过阈值返回 ErrRateLimited。
func (m *Manager) checkRateLimit(ctx context.Context, ip string) (int, error) {
	if m == nil || m.limit <= 0 || strings.TrimSpace(ip) == "" {
		return -1, nil
	}

	if m.limiter == nil {
		return -1, nil
	}

	key := fmt.Sprintf("%s:ip:%s", m.prefix, ip)
	result, err := m.limiter.Allow(ctx, key, m.limit, m.window)
	if err != nil {
		return 0, fmt.Errorf("captcha rate limit: %w", err)
	}
	if !result.Allowed {
		return 0, ErrRateLimited
	}

	return result.Remaining, nil
}
