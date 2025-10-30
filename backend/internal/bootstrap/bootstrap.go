/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 20:51:28
 * @FilePath: \electron-go-app\backend\internal\bootstrap\bootstrap.go
 * @LastEditTime: 2025-10-28 14:03:03
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
	promptcommentsvc "electron-go-app/backend/internal/service/promptcomment"
	publicpromptsvc "electron-go-app/backend/internal/service/publicprompt"
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
	publicPromptRepo := repository.NewPublicPromptRepository(resources.DBConn())
	promptCommentRepo := repository.NewPromptCommentRepository(resources.DBConn())

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

	var commentLimiter ratelimit.Limiter
	if resources.Redis != nil {
		commentLimiter = ratelimit.NewRedisLimiter(resources.Redis, "prompt_comment")
	} else {
		commentLimiter = ratelimit.NewMemoryLimiter()
	}

	var publicPromptLimiter ratelimit.Limiter
	if resources.Redis != nil {
		publicPromptLimiter = ratelimit.NewRedisLimiter(resources.Redis, "public_prompt")
	} else {
		publicPromptLimiter = ratelimit.NewMemoryLimiter()
	}
	var freeTierLimiter ratelimit.Limiter
	if resources.Redis != nil {
		freeTierLimiter = ratelimit.NewRedisLimiter(resources.Redis, "prompt_free")
	} else {
		freeTierLimiter = ratelimit.NewMemoryLimiter()
	}
	// 本地模式下禁用所有限流器，方便开发与测试。
	if isLocalMode {
		logger.Infow("disabling rate limiters in local mode")
		limiter = nil
		promptLimiter = nil
		commentLimiter = nil
		publicPromptLimiter = nil
		freeTierLimiter = nil
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

	// Prompt 服务与 Handler 较为复杂，涉及关键词管理、工作空间、持久化队列等。
	promptCfg := loadPromptConfig(logger, isLocalMode)
	// 模型服务与 Handler 负责模型凭据的管理与测试连接。
	modelService := modelsvc.NewService(modelRepo, userRepo)
	// 构建 Prompt 工作台服务，并注入关键词上限、限流配置等依赖。
	promptService, err := promptsvc.NewServiceWithConfig(promptRepo, keywordRepo, modelService, workspaceStore, persistenceQueue, logger, freeTierLimiter, promptCfg)
	if err != nil {
		return nil, fmt.Errorf("init prompt service: %w", err)
	}
	var builtinModels []modelsvc.Credential
	if info := promptService.FreeTierInfo(); info != nil && strings.TrimSpace(info.Alias) != "" {
		daily := info.DailyQuota
		displayName := strings.TrimSpace(info.DisplayName)
		if displayName == "" {
			displayName = strings.TrimSpace(info.Alias)
		}
		actualModel := strings.TrimSpace(info.ActualModel)
		if actualModel == "" {
			actualModel = strings.TrimSpace(info.Alias)
		}
		builtinModels = append(builtinModels, modelsvc.Credential{
			Provider:    info.Provider,
			ModelKey:    info.Alias,
			DisplayName: displayName,
			ActualModel: actualModel,
			Status:      "enabled",
			IsBuiltin:   true,
			DailyQuota:  &daily,
			CreatedAt:   time.Time{},
			UpdatedAt:   time.Time{},
		})
	}
	var freeTierUsage handler.FreeTierUsageFunc
	if promptService != nil {
		freeTierUsage = promptService.FreeTierUsage
	}
	modelHandler := handler.NewModelHandler(modelService, builtinModels, freeTierUsage)
	promptRateLimit := loadPromptRateLimit(logger)
	promptHandler := handler.NewPromptHandler(promptService, promptLimiter, promptRateLimit)
	// 构建 Prompt 评论服务与 Handler，注入评论审核配置与限流器。
	commentCfg := loadPromptCommentConfig(logger)
	var commentAudit promptcommentsvc.AuditFunc
	if promptService != nil {
		commentAudit = promptService.AuditCommentContent
	}
	commentService := promptcommentsvc.NewService(promptCommentRepo, promptRepo, userRepo, commentAudit, logger, commentCfg)
	commentRateLimit := loadPromptCommentRateLimit(logger)
	commentHandler := handler.NewPromptCommentHandler(commentService, commentLimiter, commentRateLimit)
	// 公开 Prompt 服务与 Handler 仅负责公开库的查询功能。
	publicPromptRate := loadPublicPromptRateLimit(logger)
	publicPromptListCfg := loadPublicPromptListConfig(logger)
	publicPromptService := publicpromptsvc.NewServiceWithConfig(publicPromptRepo, resources.DBConn(), logger, !isLocalMode, publicPromptListCfg, resources.Redis)
	publicPromptHandler := handler.NewPublicPromptHandler(publicPromptService, publicPromptLimiter, publicPromptRate)

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
	if publicPromptService != nil {
		publicPromptService.StartVisitFlushWorker(ctx)
		publicPromptService.StartScoreRefreshWorker(ctx)
	}
	// 更新日志服务与 Handler 相对简单，主要负责变更条目的查询。
	changelogService := changelogsrv.NewService(changelogRepo)
	changelogHandler := handler.NewChangelogHandler(changelogService, modelService)

	// 上传服务目前仅用于头像上传，默认写入 public/avatars，可在本地模式下改为用户数据目录。
	avatarStorage := filepath.Join("public", "avatars")
	if isLocalMode {
		dbDir := filepath.Dir(cfg.LocalUser.DBPath)
		if dbDir == "" {
			dbDir = "."
		}
		avatarStorage = filepath.Join(dbDir, "avatars")
	}
	uploadHandler := handler.NewUploadHandler(avatarStorage)
	staticFS := server.NewHybridStaticFS("public", avatarStorage)

	// 构建路由与中间件。
	var authenticator middleware.Authenticator
	if isLocalMode {
		logger.Infow("initialising offline auth middleware", "user_id", cfg.LocalUser.UserID, "is_admin", cfg.LocalUser.IsAdmin)
		authenticator = middleware.NewOfflineAuthMiddleware(cfg.LocalUser.UserID, cfg.LocalUser.IsAdmin)
	} else {
		logger.Infow("initialising jwt auth middleware")
		authenticator = middleware.NewAuthMiddleware(cfg.JWTSecret)
	}
	router := server.NewRouter(server.RouterOptions{
		AuthHandler:          authHandler,
		UserHandler:          userHandler,
		UploadHandler:        uploadHandler,
		ModelHandler:         modelHandler,
		ChangelogHandler:     changelogHandler,
		PromptHandler:        promptHandler,
		PromptCommentHandler: commentHandler,
		PublicPromptHandler:  publicPromptHandler,
		AuthMW:               authenticator,
		IPGuard:              ipGuard,
		IPGuardHandler:       ipGuardHandler,
		StaticFS:             staticFS,
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
	saveLimit := parseIntEnv("PROMPT_SAVE_LIMIT", handler.DefaultSaveLimit, logger)
	saveWindow := parseDurationEnv("PROMPT_SAVE_WINDOW", handler.DefaultSaveWindow, logger)
	publishLimit := parseIntEnv("PROMPT_PUBLISH_LIMIT", handler.DefaultPublishLimit, logger)
	publishWindow := parseDurationEnv("PROMPT_PUBLISH_WINDOW", handler.DefaultPublishWindow, logger)
	return handler.PromptRateLimit{
		InterpretLimit:  interpretLimit,
		InterpretWindow: interpretWindow,
		GenerateLimit:   generateLimit,
		GenerateWindow:  generateWindow,
		SaveLimit:       saveLimit,
		SaveWindow:      saveWindow,
		PublishLimit:    publishLimit,
		PublishWindow:   publishWindow,
	}
}

// loadPublicPromptRateLimit 读取公共 Prompt 库的限流配置。
func loadPublicPromptRateLimit(logger *zap.SugaredLogger) handler.PublicPromptRateLimit {
	submitLimit := parseIntEnv("PUBLIC_PROMPT_SUBMIT_LIMIT", handler.DefaultPublicSubmitLimit, logger)
	submitWindow := parseDurationEnv("PUBLIC_PROMPT_SUBMIT_WINDOW", handler.DefaultPublicSubmitWindow, logger)
	downloadLimit := parseIntEnv("PUBLIC_PROMPT_DOWNLOAD_LIMIT", handler.DefaultPublicDownloadLimit, logger)
	downloadWindow := parseDurationEnv("PUBLIC_PROMPT_DOWNLOAD_WINDOW", handler.DefaultPublicDownloadWindow, logger)
	return handler.PublicPromptRateLimit{
		SubmitLimit:    submitLimit,
		SubmitWindow:   submitWindow,
		DownloadLimit:  downloadLimit,
		DownloadWindow: downloadWindow,
	}
}

// loadPublicPromptListConfig 读取公共 Prompt 列表分页配置。
func loadPublicPromptListConfig(logger *zap.SugaredLogger) publicpromptsvc.Config {
	cfg := publicpromptsvc.Config{
		DefaultPageSize: parseIntEnv("PUBLIC_PROMPT_LIST_PAGE_SIZE", publicpromptsvc.DefaultListPageSize, logger),
		MaxPageSize:     parseIntEnv("PUBLIC_PROMPT_LIST_MAX_PAGE_SIZE", publicpromptsvc.DefaultListMaxPageSize, logger),
	}
	cfg.Visit = loadPublicPromptVisitConfig(logger)
	cfg.Score = loadPublicPromptScoreConfig(logger)
	return cfg
}

// loadPublicPromptVisitConfig 读取公共库访问量统计配置。
func loadPublicPromptVisitConfig(logger *zap.SugaredLogger) publicpromptsvc.VisitConfig {
	return publicpromptsvc.VisitConfig{
		Enabled:       parseBoolEnv("PUBLIC_PROMPT_VISIT_ENABLED", true),
		BufferKey:     strings.TrimSpace(os.Getenv("PUBLIC_PROMPT_VISIT_BUFFER_KEY")),
		GuardPrefix:   strings.TrimSpace(os.Getenv("PUBLIC_PROMPT_VISIT_GUARD_PREFIX")),
		GuardTTL:      parseDurationEnv("PUBLIC_PROMPT_VISIT_GUARD_TTL", time.Minute, logger),
		FlushInterval: parseDurationEnv("PUBLIC_PROMPT_VISIT_FLUSH_INTERVAL", time.Minute, logger),
		FlushBatch:    parseIntEnv("PUBLIC_PROMPT_VISIT_FLUSH_BATCH", 128, logger),
		FlushLockKey:  strings.TrimSpace(os.Getenv("PUBLIC_PROMPT_VISIT_FLUSH_LOCK_KEY")),
		FlushLockTTL:  parseDurationEnv("PUBLIC_PROMPT_VISIT_FLUSH_LOCK_TTL", 10*time.Second, logger),
	}
}

// loadPublicPromptScoreConfig 读取公共库质量评分的权重配置，支持通过环境变量自定义排行策略。
func loadPublicPromptScoreConfig(logger *zap.SugaredLogger) publicpromptsvc.ScoreConfig {
	return publicpromptsvc.ScoreConfig{
		Enabled:         parseBoolEnv("PUBLIC_PROMPT_SCORE_ENABLED", true),
		LikeWeight:      parseFloatEnv("PUBLIC_PROMPT_SCORE_LIKE_WEIGHT", 0, logger),
		DownloadWeight:  parseFloatEnv("PUBLIC_PROMPT_SCORE_DOWNLOAD_WEIGHT", 0, logger),
		VisitWeight:     parseFloatEnv("PUBLIC_PROMPT_SCORE_VISIT_WEIGHT", 0, logger),
		RecencyWeight:   parseFloatEnv("PUBLIC_PROMPT_SCORE_RECENCY_WEIGHT", 0, logger),
		RecencyHalfLife: parseDurationEnv("PUBLIC_PROMPT_SCORE_RECENCY_HALF_LIFE", 0, logger),
		BaseScore:       parseFloatEnv("PUBLIC_PROMPT_SCORE_BASE", 0, logger),
		RefreshInterval: parseDurationEnv("PUBLIC_PROMPT_SCORE_REFRESH_INTERVAL", 0, logger),
		RefreshBatch:    parseIntEnv("PUBLIC_PROMPT_SCORE_REFRESH_BATCH", 0, logger),
	}
}

// loadPromptCommentConfig 读取评论模块的分页与审核配置。
func loadPromptCommentConfig(logger *zap.SugaredLogger) promptcommentsvc.Config {
	defaultPage := parseIntEnv("PROMPT_COMMENT_PAGE_SIZE", 10, logger)
	maxPage := parseIntEnv("PROMPT_COMMENT_MAX_PAGE_SIZE", 60, logger)
	maxLength := parseIntEnv("PROMPT_COMMENT_MAX_LENGTH", 1000, logger)
	if maxLength <= 0 {
		maxLength = 1000
	}
	requireApproval := parseBoolEnv("PROMPT_COMMENT_REQUIRE_APPROVAL", false)
	return promptcommentsvc.Config{
		DefaultPageSize: defaultPage,
		MaxPageSize:     maxPage,
		RequireApproval: requireApproval,
		MaxBodyLength:   maxLength,
	}
}

// loadPromptCommentRateLimit 读取评论创建的限流参数。
func loadPromptCommentRateLimit(logger *zap.SugaredLogger) handler.PromptCommentRateLimit {
	limit := parseIntEnv("PROMPT_COMMENT_RATE_LIMIT", 12, logger)
	window := parseDurationEnv("PROMPT_COMMENT_RATE_WINDOW", 30*time.Second, logger)
	return handler.PromptCommentRateLimit{
		CreateLimit:  limit,
		CreateWindow: window,
	}
}

// loadPromptConfig 汇总 Prompt 模块使用到的配置项。
func loadPromptConfig(logger *zap.SugaredLogger, isLocal bool) promptsvc.Config {
	auditEnabled := parseBoolEnv("PROMPT_AUDIT_ENABLED", !isLocal)
	if isLocal {
		auditEnabled = false
	}
	auditProvider := strings.TrimSpace(os.Getenv("PROMPT_AUDIT_PROVIDER"))
	auditModelKey := strings.TrimSpace(os.Getenv("PROMPT_AUDIT_MODEL_KEY"))
	auditAPIKey := strings.TrimSpace(os.Getenv("PROMPT_AUDIT_API_KEY"))
	auditBaseURL := strings.TrimSpace(os.Getenv("PROMPT_AUDIT_BASE_URL"))
	if auditEnabled && auditAPIKey == "" {
		logger.Warnw("audit enabled but API key missing, disabling", "provider", auditProvider)
		auditEnabled = false
	}
	freeTierEnabled := parseBoolEnv("PROMPT_FREE_TIER_ENABLED", !isLocal)
	if isLocal {
		freeTierEnabled = false
	}
	freeTierProvider := strings.TrimSpace(os.Getenv("PROMPT_FREE_TIER_PROVIDER"))
	freeTierAlias := strings.TrimSpace(os.Getenv("PROMPT_FREE_TIER_MODEL_KEY"))
	freeTierActual := strings.TrimSpace(os.Getenv("PROMPT_FREE_TIER_ACTUAL_MODEL"))
	freeTierDisplay := strings.TrimSpace(os.Getenv("PROMPT_FREE_TIER_DISPLAY_NAME"))
	freeTierAPIKey := strings.TrimSpace(os.Getenv("PROMPT_FREE_TIER_API_KEY"))
	if freeTierAPIKey == "" {
		freeTierAPIKey = auditAPIKey
	}
	freeTierBaseURL := strings.TrimSpace(os.Getenv("PROMPT_FREE_TIER_BASE_URL"))
	freeTierLimit := parseIntEnv("PROMPT_FREE_TIER_DAILY_LIMIT", promptsvc.DefaultFreeTierDailyLimit, logger)
	freeTierWindow := parseDurationEnv("PROMPT_FREE_TIER_WINDOW", promptsvc.DefaultFreeTierWindow, logger)
	return promptsvc.Config{
		KeywordLimit:        parseIntEnv("PROMPT_KEYWORD_LIMIT", promptsvc.DefaultKeywordLimit, logger),
		KeywordMaxLength:    parseIntEnv("PROMPT_KEYWORD_MAX_LENGTH", promptsvc.DefaultKeywordMaxLength, logger),
		TagLimit:            parseIntEnv("PROMPT_TAG_LIMIT", promptsvc.DefaultTagLimit, logger),
		TagMaxLength:        parseIntEnv("PROMPT_TAG_MAX_LENGTH", promptsvc.DefaultTagMaxLength, logger),
		DefaultListPageSize: parseIntEnv("PROMPT_LIST_PAGE_SIZE", promptsvc.DefaultPromptListPageSize, logger),
		MaxListPageSize:     parseIntEnv("PROMPT_LIST_MAX_PAGE_SIZE", promptsvc.DefaultPromptListMaxPageSize, logger),
		UseFullTextSearch:   parseBoolEnv("PROMPT_USE_FULLTEXT", false),
		ExportDirectory:     strings.TrimSpace(os.Getenv("PROMPT_EXPORT_DIR")),
		VersionRetention:    parseIntEnv("PROMPT_VERSION_KEEP_LIMIT", promptsvc.DefaultVersionRetentionLimit, logger),
		ImportBatchSize:     parseIntEnv("PROMPT_IMPORT_BATCH_SIZE", promptsvc.DefaultImportBatchSize, logger),
		Audit: promptsvc.AuditConfig{
			Enabled:  auditEnabled,
			Provider: auditProvider,
			ModelKey: auditModelKey,
			APIKey:   auditAPIKey,
			BaseURL:  auditBaseURL,
		},
		FreeTier: promptsvc.FreeTierConfig{
			Enabled:     freeTierEnabled,
			Provider:    freeTierProvider,
			Alias:       freeTierAlias,
			ActualModel: freeTierActual,
			DisplayName: freeTierDisplay,
			APIKey:      freeTierAPIKey,
			BaseURL:     freeTierBaseURL,
			DailyQuota:  freeTierLimit,
			Window:      freeTierWindow,
		},
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

// parseFloatEnv 解析浮点型环境变量，失败时记录告警并返回默认值，避免配置错误影响服务。
func parseFloatEnv(key string, defaultValue float64, logger *zap.SugaredLogger) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		logger.Warnw("invalid float env, fallback to default", "key", key, "value", raw, "default", defaultValue, "error", err)
		return defaultValue
	}
	return value
}
