/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 20:22:45
 * @FilePath: \electron-go-app\backend\internal\infra\captcha\config.go
 * @LastEditTime: 2025-10-09 20:22:52
 */
package captcha

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	client "electron-go-app/backend/internal/infra/client"
)

// 环境变量字段名称常量，避免散落的硬编码。
const (
	envCaptchaEnabled         = "CAPTCHA_ENABLED"
	envCaptchaPrefix          = "CAPTCHA_PREFIX"
	envCaptchaTTL             = "CAPTCHA_TTL"
	envCaptchaWidth           = "CAPTCHA_WIDTH"
	envCaptchaHeight          = "CAPTCHA_HEIGHT"
	envCaptchaLength          = "CAPTCHA_LENGTH"
	envCaptchaMaxSkew         = "CAPTCHA_MAX_SKEW"
	envCaptchaDotCount        = "CAPTCHA_DOT_COUNT"
	envCaptchaRateLimit       = "CAPTCHA_RATE_LIMIT_PER_MIN"
	envCaptchaRateLimitWindow = "CAPTCHA_RATE_LIMIT_WINDOW"

	envCaptchaConfigDataID = "CAPTCHA_CONFIG_DATA_ID"
	envCaptchaConfigGroup  = "CAPTCHA_CONFIG_GROUP"
)

const defaultCaptchaGroup = "DEFAULT_GROUP"

type nacosCaptchaConfig struct {
	Enabled         bool    `json:"enabled"`
	Prefix          string  `json:"prefix"`
	TTL             string  `json:"ttl"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	Length          int     `json:"length"`
	MaxSkew         float64 `json:"max_skew"`
	DotCount        int     `json:"dot_count"`
	RateLimitPerMin int     `json:"rate_limit_per_min"`
	RateLimitWindow string  `json:"rate_limit_window"`
}

// LoadOptions 优先从 Nacos 读取验证码配置，未指定时退回环境变量。
func LoadOptions(ctx context.Context) (Options, bool, *WatchConfig, error) {
	dataID := strings.TrimSpace(os.Getenv(envCaptchaConfigDataID))
	if dataID != "" {
		group := strings.TrimSpace(os.Getenv(envCaptchaConfigGroup))
		if group == "" {
			group = defaultCaptchaGroup
		}

		opts, err := client.NewDefaultNacosOptions()
		if err != nil {
			return Options{}, false, nil, fmt.Errorf("build nacos options: %w", err)
		}

		content, err := client.FetchNacosConfig(ctx, opts, dataID, group)
		if err != nil {
			return Options{}, false, nil, fmt.Errorf("fetch captcha config: %w", err)
		}

		cfg, err := parseCaptchaConfigJSON(content)
		if err != nil {
			return Options{}, false, nil, err
		}

		interval := defaultCaptchaPollInterval
		if raw := strings.TrimSpace(os.Getenv(envCaptchaPollInterval)); raw != "" {
			parsedInterval, parseErr := time.ParseDuration(raw)
			if parseErr != nil {
				return Options{}, false, nil, fmt.Errorf("parse %s: %w", envCaptchaPollInterval, parseErr)
			}
			if parsedInterval <= 0 {
				return Options{}, false, nil, fmt.Errorf("%s must be greater than zero", envCaptchaPollInterval)
			}
			interval = parsedInterval
		}

		watch := &WatchConfig{
			Nacos:        opts,
			DataID:       dataID,
			Group:        group,
			PollInterval: interval,
			LastRaw:      content,
		}

		if !cfg.Enabled {
			return Options{}, false, watch, nil
		}

		parsed, err := buildOptionsFromPayload(cfg)
		if err != nil {
			return Options{}, false, nil, err
		}

		return parsed, true, watch, nil
	}

	opts, enabled, err := LoadOptionsFromEnv()
	return opts, enabled, nil, err
}

// LoadOptionsFromEnv 解析环境变量并返回 Options，同时指示功能是否开启。
// 当声明启用验证码时，一旦解析失败会返回错误，方便在启动阶段及时终止。
func LoadOptionsFromEnv() (Options, bool, error) {
	rawEnabled := strings.TrimSpace(os.Getenv(envCaptchaEnabled))
	if rawEnabled == "" {
		return Options{}, false, nil
	}

	enabled := isTruthy(rawEnabled)
	if !enabled {
		return Options{}, false, nil
	}

	opts := Options{}

	if prefix := strings.TrimSpace(os.Getenv(envCaptchaPrefix)); prefix != "" {
		opts.Prefix = prefix
	}

	if rawTTL := strings.TrimSpace(os.Getenv(envCaptchaTTL)); rawTTL != "" {
		ttl, err := time.ParseDuration(rawTTL)
		if err != nil {
			return Options{}, false, fmt.Errorf("parse %s: %w", envCaptchaTTL, err)
		}
		opts.TTL = ttl
	}

	if rawWidth := strings.TrimSpace(os.Getenv(envCaptchaWidth)); rawWidth != "" {
		width, err := strconv.Atoi(rawWidth)
		if err != nil {
			return Options{}, false, fmt.Errorf("parse %s: %w", envCaptchaWidth, err)
		}
		opts.Width = width
	}

	if rawHeight := strings.TrimSpace(os.Getenv(envCaptchaHeight)); rawHeight != "" {
		height, err := strconv.Atoi(rawHeight)
		if err != nil {
			return Options{}, false, fmt.Errorf("parse %s: %w", envCaptchaHeight, err)
		}
		opts.Height = height
	}

	if rawLength := strings.TrimSpace(os.Getenv(envCaptchaLength)); rawLength != "" {
		length, err := strconv.Atoi(rawLength)
		if err != nil {
			return Options{}, false, fmt.Errorf("parse %s: %w", envCaptchaLength, err)
		}
		opts.Length = length
	}

	if rawSkew := strings.TrimSpace(os.Getenv(envCaptchaMaxSkew)); rawSkew != "" {
		skew, err := strconv.ParseFloat(rawSkew, 64)
		if err != nil {
			return Options{}, false, fmt.Errorf("parse %s: %w", envCaptchaMaxSkew, err)
		}
		opts.MaxSkew = skew
	}

	if rawDots := strings.TrimSpace(os.Getenv(envCaptchaDotCount)); rawDots != "" {
		dots, err := strconv.Atoi(rawDots)
		if err != nil {
			return Options{}, false, fmt.Errorf("parse %s: %w", envCaptchaDotCount, err)
		}
		opts.DotCount = dots
	}

	if rawRate := strings.TrimSpace(os.Getenv(envCaptchaRateLimit)); rawRate != "" {
		rate, err := strconv.Atoi(rawRate)
		if err != nil {
			return Options{}, false, fmt.Errorf("parse %s: %w", envCaptchaRateLimit, err)
		}
		opts.RateLimitPerMin = rate
	}

	if rawWindow := strings.TrimSpace(os.Getenv(envCaptchaRateLimitWindow)); rawWindow != "" {
		window, err := time.ParseDuration(rawWindow)
		if err != nil {
			return Options{}, false, fmt.Errorf("parse %s: %w", envCaptchaRateLimitWindow, err)
		}
		opts.RateLimitWindow = window
	}

	return opts, true, nil
}

// isTruthy 统一处理字符串形式的布尔值，兼容常见写法。
func isTruthy(v string) bool {
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseCaptchaConfigJSON(content string) (nacosCaptchaConfig, error) {
	var payload nacosCaptchaConfig
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return nacosCaptchaConfig{}, fmt.Errorf("parse captcha config: %w", err)
	}
	return payload, nil
}

func buildOptionsFromPayload(payload nacosCaptchaConfig) (Options, error) {
	opts := Options{
		Prefix:          payload.Prefix,
		Width:           payload.Width,
		Height:          payload.Height,
		Length:          payload.Length,
		MaxSkew:         payload.MaxSkew,
		DotCount:        payload.DotCount,
		RateLimitPerMin: payload.RateLimitPerMin,
	}

	if strings.TrimSpace(payload.TTL) != "" {
		ttl, err := time.ParseDuration(payload.TTL)
		if err != nil {
			return Options{}, fmt.Errorf("parse ttl: %w", err)
		}
		opts.TTL = ttl
	}

	if strings.TrimSpace(payload.RateLimitWindow) != "" {
		window, err := time.ParseDuration(payload.RateLimitWindow)
		if err != nil {
			return Options{}, fmt.Errorf("parse rate_limit_window: %w", err)
		}
		opts.RateLimitWindow = window
	}

	return opts, nil
}
