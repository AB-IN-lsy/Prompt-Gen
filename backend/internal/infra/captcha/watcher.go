package captcha

import (
	"context"
	"time"

	client "electron-go-app/backend/internal/infra/client"

	"go.uber.org/zap"
)

const (
	defaultCaptchaPollInterval = 30 * time.Second
	envCaptchaPollInterval     = "CAPTCHA_CONFIG_POLL_INTERVAL"
)

// WatchConfig 描述从 Nacos 拉取配置所需的信息。
type WatchConfig struct {
	Nacos        client.NacosOptions
	DataID       string
	Group        string
	PollInterval time.Duration
	LastRaw      string
}

// StartWatcher 定期从 Nacos 拉取验证码配置并更新动态管理器。
func StartWatcher(ctx context.Context, cfg WatchConfig, manager *DynamicManager, logger *zap.SugaredLogger) {
	interval := cfg.PollInterval
	if interval <= 0 {
		interval = defaultCaptchaPollInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	lastRaw := cfg.LastRaw

	for {
		select {
		case <-ctx.Done():
			logger.Infow("captcha watcher stopped", "reason", "context cancelled")
			return
		case <-ticker.C:
			refreshCaptchaConfig(ctx, cfg, manager, logger, &lastRaw)
		}
	}
}

func refreshCaptchaConfig(ctx context.Context, cfg WatchConfig, manager *DynamicManager, logger *zap.SugaredLogger, lastRaw *string) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	raw, err := client.FetchNacosConfig(reqCtx, cfg.Nacos, cfg.DataID, cfg.Group)
	if err != nil {
		logger.Warnw("fetch captcha config failed", "error", err)
		return
}

	if raw == *lastRaw {
		return
	}

	payload, err := parseCaptchaConfigJSON(raw)
	if err != nil {
		logger.Warnw("parse captcha config failed", "error", err)
		return
	}

	if !payload.Enabled {
		if err := manager.Swap(Options{}, false); err != nil {
			logger.Warnw("disable captcha failed", "error", err)
			return
		}
		logger.Infow("captcha disabled via nacos update")
		*lastRaw = raw
		return
	}

	opts, err := buildOptionsFromPayload(payload)
	if err != nil {
		logger.Warnw("build captcha options failed", "error", err)
		return
	}

	if err := manager.Swap(opts, true); err != nil {
		logger.Warnw("apply captcha config failed", "error", err)
		return
	}

	logger.Infow("captcha config updated", "data_id", cfg.DataID, "group", cfg.Group)
	*lastRaw = raw
}

