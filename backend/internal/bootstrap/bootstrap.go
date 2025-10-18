/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 20:51:28
 * @FilePath: \electron-go-app\backend\internal\bootstrap\bootstrap.go
 * @LastEditTime: 2025-10-14 20:38:19
 */
package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"electron-go-app/backend/internal/app"
	"electron-go-app/backend/internal/config"
	"electron-go-app/backend/internal/handler"
	"electron-go-app/backend/internal/infra/captcha"
	"electron-go-app/backend/internal/infra/email"
	promptinfra "electron-go-app/backend/internal/infra/prompt"
	"electron-go-app/backend/internal/infra/ratelimit"
	"electron-go-app/backend/internal/infra/token"
	"electron-go-app/backend/internal/middleware"
	"electron-go-app/backend/internal/repository"
	"electron-go-app/backend/internal/server"
	authsvc "electron-go-app/backend/internal/service/auth"
	changelogsrv "electron-go-app/backend/internal/service/changelog"
	modelsvc "electron-go-app/backend/internal/service/model"
	promptsvc "electron-go-app/backend/internal/service/prompt"
	usersvc "electron-go-app/backend/internal/service/user"

	"go.uber.org/zap"
)

type RuntimeConfig struct {
	Port       string
	JWTSecret  string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	Mode       string
	LocalUser  config.LocalRuntime
}

type Application struct {
	Resources *app.Resources
	AuthSvc   *authsvc.Service
	UserSvc   *usersvc.Service
	ModelSvc  *modelsvc.Service
	PromptSvc *promptsvc.Service
	Changelog *changelogsrv.Service
	Router    http.Handler
}

// BuildApplication 将底层资源 (数据库、Redis 等) 与业务配置装配成完整的 HTTP 应用：
// 1. 初始化仓储与领域服务；2. 注入限流、邮件、验证码等外围依赖；3. 构造路由与中间件，返回可直接启动的应用实体。
func BuildApplication(ctx context.Context, logger *zap.SugaredLogger, resources *app.Resources, cfg RuntimeConfig) (*Application, error) {
	// 创建核心仓储：用户信息与邮箱验证令牌都依赖 MySQL/Gorm.
	userRepo := repository.NewUserRepository(resources.DBConn())
	verificationRepo := repository.NewEmailVerificationRepository(resources.DBConn())
	tokens := token.NewJWTManager(cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL)
	modelRepo := repository.NewModelCredentialRepository(resources.DBConn())
	changelogRepo := repository.NewChangelogRepository(resources.DBConn())
	promptRepo := repository.NewPromptRepository(resources.DBConn())
	keywordRepo := repository.NewKeywordRepository(resources.DBConn())

	isLocalMode := strings.EqualFold(cfg.Mode, config.ModeLocal)

	var (
		workspaceStore   promptsvc.WorkspaceStore
		persistenceQueue promptsvc.PersistenceQueue
	)
	if resources.Redis != nil {
		workspaceStore = promptinfra.NewWorkspaceStore(resources.Redis)
		persistenceQueue = promptinfra.NewPersistenceQueue(resources.Redis, "")
	}

	// 刷新令牌优先落在 Redis，便于服务重启后继续验证。
	var refreshStore authsvc.RefreshTokenStore
	if resources.Redis != nil {
		refreshStore = token.NewRedisRefreshTokenStore(resources.Redis, "")
	} else {
		refreshStore = token.NewMemoryRefreshTokenStore()
		logger.Infow("using in-memory refresh token store; tokens won't persist across restarts")
	}

	// 验证码管理器是可选的：未启用时返回 nil，后续 Handler 会自检。
	captchaManager, err := initCaptchaManager(ctx, resources, logger)
	if err != nil {
		return nil, err
	}

	// 邮件发送同样是可选功能；若未配置 SMTP，将退回日志打印模式。
	emailSender, err := initEmailSender(logger)
	if err != nil {
		return nil, err
	}

	// 统一使用限流模块控制邮箱验证请求频率。
	var limiter ratelimit.Limiter
	if resources.Redis != nil {
		limiter = ratelimit.NewRedisLimiter(resources.Redis, "verify_email")
	} else {
		limiter = ratelimit.NewMemoryLimiter()
	}
	var promptLimiter ratelimit.Limiter
	if resources.Redis != nil {
		promptLimiter = ratelimit.NewRedisLimiter(resources.Redis, "prompt")
	} else {
		promptLimiter = ratelimit.NewMemoryLimiter()
	}

	// 支持通过环境变量自定义验证邮件的频率限制。
	verificationLimit, verificationWindow := loadEmailVerificationRateConfig(logger)

	// 汇总依赖，构建鉴权服务与 Handler。
	// 鉴权服务依赖较多，需协调验证码、邮件、令牌、用户等多个模块。
	authService := authsvc.NewService(userRepo, verificationRepo, tokens, refreshStore, captchaManager, emailSender)
	authHandler := handler.NewAuthHandler(authService, limiter, verificationLimit, verificationWindow)

	// 用户服务与 Handler 相对简单，主要负责用户信息的 CRUD。
	userService := usersvc.NewService(userRepo, modelRepo)
	userHandler := handler.NewUserHandler(userService)

	// 模型服务与 Handler 负责模型凭据的管理与测试连接。
	modelService := modelsvc.NewService(modelRepo, userRepo)
	modelHandler := handler.NewModelHandler(modelService)

	// Prompt 服务与 Handler 较为复杂，涉及关键词管理、工作空间、持久化队列等。
	promptCfg := loadPromptConfig(logger)
	// 构建 Prompt 工作台服务，并注入关键词上限、限流配置等依赖。
	promptService := promptsvc.NewServiceWithConfig(promptRepo, keywordRepo, modelService, workspaceStore, persistenceQueue, logger, promptCfg)
	promptRateLimit := loadPromptRateLimit(logger)
	promptHandler := handler.NewPromptHandler(promptService, promptLimiter, promptRateLimit)

	// IP 防护中间件也是可选的，且强依赖 Redis。
	ipGuardCfg := loadIPGuardConfig(logger)
	var ipGuard *middleware.IPGuardMiddleware
	if ipGuardCfg.Enabled {
		if resources.Redis != nil {
			ipGuard = middleware.NewIPGuardMiddleware(resources.Redis, ipGuardCfg)
		} else {
			logger.Warnw("ip guard enabled but redis unavailable, feature disabled")
		}
	}
	var ipGuardHandler *handler.IPGuardHandler
	if ipGuard != nil {
		ipGuardHandler = handler.NewIPGuardHandler(ipGuard)
	}

	// 启动后台持久化任务（若 Redis 可用）。
	if workspaceStore != nil && persistenceQueue != nil {
		promptService.StartPersistenceWorker(ctx, 0)
	}
	// 更新日志服务与 Handler 相对简单，主要负责变更条目的查询。
	changelogService := changelogsrv.NewService(changelogRepo)
	changelogHandler := handler.NewChangelogHandler(changelogService, modelService)

	// 上传服务目前仅用于头像上传，且不依赖任何持久化存储。
	// 文件直接存储在本地的 public 目录下，后续可扩展至云存储。
	uploadStorage := filepath.Join("public", "avatars")
	uploadHandler := handler.NewUploadHandler(uploadStorage)

	// 构建路由与中间件。
	authMiddleware := middleware.NewAuthMiddleware(cfg.JWTSecret)
	if isLocalMode {
		authMiddleware = nil
	}
	var offlineAuth middleware.Authenticator
	if isLocalMode {
		offlineAuth = middleware.NewOfflineAuthMiddleware(cfg.LocalUser.UserID, cfg.LocalUser.IsAdmin)
	}

	router := server.NewRouter(server.RouterOptions{
		AuthHandler:      authHandler,
		UserHandler:      userHandler,
		UploadHandler:    uploadHandler,
		ModelHandler:     modelHandler,
		ChangelogHandler: changelogHandler,
		PromptHandler:    promptHandler,
		AuthMW: func() middleware.Authenticator {
			if isLocalMode {
				return offlineAuth
			}
			return authMiddleware
		}(),
		IPGuard:        ipGuard,
		IPGuardHandler: ipGuardHandler,
	})

	return &Application{
		Resources: resources,
		AuthSvc:   authService,
		UserSvc:   userService,
		ModelSvc:  modelService,
		PromptSvc: promptService,
		Changelog: changelogService,
		Router:    router,
	}, nil
}

func initCaptchaManager(ctx context.Context, resources *app.Resources, logger *zap.SugaredLogger) (authsvc.CaptchaManager, error) {
	opts, enabled, watchCfg, err := captcha.LoadOptions(ctx)
	if err != nil {
		logger.Errorw("load captcha config failed", "error", err)
		return nil, fmt.Errorf("load captcha config: %w", err)
	}

	if !enabled && watchCfg == nil {
		logger.Infow("captcha disabled")
		return nil, nil
	}

	if resources.Redis == nil {
		return nil, fmt.Errorf("captcha enabled but redis not configured")
	}

	prefix := opts.Prefix
	if strings.TrimSpace(prefix) == "" {
		prefix = "captcha"
	}
	limiter := ratelimit.NewRedisLimiter(resources.Redis, fmt.Sprintf("%s_rl", prefix))
	dynamicManager := captcha.NewDynamicManager(resources.Redis, limiter)
	if err := dynamicManager.Swap(opts, enabled); err != nil {
		return nil, fmt.Errorf("init captcha manager: %w", err)
	}

	if watchCfg != nil {
		go captcha.StartWatcher(ctx, *watchCfg, dynamicManager, logger)
		logger.Infow("captcha watcher started", "data_id", watchCfg.DataID, "group", watchCfg.Group, "interval", watchCfg.PollInterval)
	}

	if dynamicManager.Enabled() {
		logger.Infow("captcha enabled", "prefix", opts.Prefix, "ttl", opts.TTL)
	} else {
		logger.Infow("captcha currently disabled")
	}

	return dynamicManager, nil
}

func initEmailSender(logger *zap.SugaredLogger) (authsvc.EmailSender, error) {
	aliyunCfg, aliyunEnabled, err := email.LoadAliyunConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("load aliyun email config: %w", err)
	}
	if aliyunEnabled {
		sender, err := email.NewAliyunSender(aliyunCfg)
		if err != nil {
			return nil, err
		}
		logger.Infow("aliyun directmail sender enabled", "account", aliyunCfg.AccountName, "region", aliyunCfg.RegionID, "endpoint", aliyunCfg.Endpoint, "addressType", aliyunCfg.AddressType)
		return sender, nil
	}

	smtpCfg, smtpEnabled, err := email.LoadSMTPConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("load smtp email config: %w", err)
	}
	if smtpEnabled {
		smtpSender, err := email.NewSender(smtpCfg)
		if err != nil {
			return nil, fmt.Errorf("init smtp email sender: %w", err)
		}
		logger.Infow("smtp email sender enabled", "host", smtpCfg.Host, "from", smtpCfg.From)
		return smtpSender, nil
	}

	logger.Infow("email sender disabled; verification emails will be logged")
	return nil, nil
}

// loadEmailVerificationRateConfig 读取邮箱验证限流配置，支持通过环境变量覆盖默认阈值。
func loadEmailVerificationRateConfig(logger *zap.SugaredLogger) (int, time.Duration) {
	// 默认每小时 5 次，可在 .env 中覆盖。
	limit := 5
	if limitStr := os.Getenv("EMAIL_VERIFICATION_LIMIT"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		} else if err != nil {
			logger.Warnw("invalid EMAIL_VERIFICATION_LIMIT, using default", "value", limitStr, "error", err)
		}
	}

	window := time.Hour
	if windowStr := os.Getenv("EMAIL_VERIFICATION_WINDOW"); windowStr != "" {
		if dur, err := time.ParseDuration(windowStr); err == nil && dur > 0 {
			window = dur
		} else if err != nil {
			logger.Warnw("invalid EMAIL_VERIFICATION_WINDOW, using default", "value", windowStr, "error", err)
		}
	}

	return limit, window
}

// loadPromptRateLimit 读取 Prompt 工作台的限流参数，若参数缺失则回退到系统默认值。
func loadPromptRateLimit(logger *zap.SugaredLogger) handler.PromptRateLimit {
	interpretLimit := parseIntEnv("PROMPT_INTERPRET_LIMIT", handler.DefaultInterpretLimit, logger)
	interpretWindow := parseDurationEnv("PROMPT_INTERPRET_WINDOW", handler.DefaultInterpretWindow, logger)
	generateLimit := parseIntEnv("PROMPT_GENERATE_LIMIT", handler.DefaultGenerateLimit, logger)
	generateWindow := parseDurationEnv("PROMPT_GENERATE_WINDOW", handler.DefaultGenerateWindow, logger)
	return handler.PromptRateLimit{
		InterpretLimit:  interpretLimit,
		InterpretWindow: interpretWindow,
		GenerateLimit:   generateLimit,
		GenerateWindow:  generateWindow,
	}
}

// loadPromptConfig 汇总 Prompt 模块使用到的配置项。
func loadPromptConfig(logger *zap.SugaredLogger) promptsvc.Config {
	return promptsvc.Config{
		KeywordLimit:        parseIntEnv("PROMPT_KEYWORD_LIMIT", promptsvc.DefaultKeywordLimit, logger),
		KeywordMaxLength:    parseIntEnv("PROMPT_KEYWORD_MAX_LENGTH", 64, logger),
		TagLimit:            parseIntEnv("PROMPT_TAG_LIMIT", promptsvc.DefaultTagLimit, logger),
		TagMaxLength:        parseIntEnv("PROMPT_TAG_MAX_LENGTH", 5, logger),
		DefaultListPageSize: parseIntEnv("PROMPT_LIST_PAGE_SIZE", 20, logger),
		MaxListPageSize:     parseIntEnv("PROMPT_LIST_MAX_PAGE_SIZE", 100, logger),
		UseFullTextSearch:   parseBoolEnv("PROMPT_USE_FULLTEXT", false),
		ExportDirectory:     strings.TrimSpace(os.Getenv("PROMPT_EXPORT_DIR")),
	}
}

// loadIPGuardConfig 读取 IP 黑名单/限流相关的配置。
func loadIPGuardConfig(logger *zap.SugaredLogger) middleware.IPGuardConfig {
	return middleware.IPGuardConfig{
		Enabled:         parseBoolEnv("IP_GUARD_ENABLED", false),
		Prefix:          strings.TrimSpace(os.Getenv("IP_GUARD_PREFIX")),
		Window:          parseDurationEnv("IP_GUARD_WINDOW", 30*time.Second, logger),
		MaxRequests:     parseIntEnv("IP_GUARD_MAX_REQUESTS", 120, logger),
		StrikeWindow:    parseDurationEnv("IP_GUARD_STRIKE_WINDOW", 10*time.Minute, logger),
		StrikeLimit:     parseIntEnv("IP_GUARD_STRIKE_LIMIT", 5, logger),
		BanTTL:          parseDurationEnv("IP_GUARD_BAN_TTL", 30*time.Minute, logger),
		HoneypotPath:    strings.TrimSpace(os.Getenv("IP_GUARD_HONEYPOT_PATH")),
		AdminScanCount:  parseIntEnv("IP_GUARD_ADMIN_SCAN_COUNT", 100, logger),
		AdminMaxEntries: parseIntEnv("IP_GUARD_ADMIN_MAX_ENTRIES", 200, logger),
	}
}

// parseIntEnv 尝试解析整型环境变量，失败时记录警告并返回默认值。
func parseIntEnv(key string, defaultValue int, logger *zap.SugaredLogger) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		logger.Warnw("invalid integer env, fallback to default", "key", key, "value", raw, "default", defaultValue, "error", err)
		return defaultValue
	}
	return value
}

// parseDurationEnv 尝试解析 duration 环境变量，失败时记录警告并返回默认值。
func parseDurationEnv(key string, defaultValue time.Duration, logger *zap.SugaredLogger) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		logger.Warnw("invalid duration env, fallback to default", "key", key, "value", raw, "default", defaultValue, "error", err)
		return defaultValue
	}
	return value
}

func parseBoolEnv(key string, defaultValue bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultValue
	}
}
