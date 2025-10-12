/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 20:51:28
 * @FilePath: \electron-go-app\backend\internal\bootstrap\bootstrap.go
 * @LastEditTime: 2025-10-09 20:51:34
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

// BuildApplication 将底层资源 (DB、Redis) 与基础配置装配成完整的 HTTP 应用：
// 1. 构建仓储、服务、Handler；2. 注入限流、邮件、验证码等外围依赖；3. 返回可直接用于启动的路由与服务指针。
func BuildApplication(ctx context.Context, logger *zap.SugaredLogger, resources *app.Resources, cfg RuntimeConfig) (*Application, error) {
	// 创建核心仓储：用户信息与邮箱验证令牌都依赖 MySQL/Gorm.
	userRepo := repository.NewUserRepository(resources.DBConn())
	verificationRepo := repository.NewEmailVerificationRepository(resources.DBConn())
	tokens := token.NewJWTManager(cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL)
	modelRepo := repository.NewModelCredentialRepository(resources.DBConn())
	changelogRepo := repository.NewChangelogRepository(resources.DBConn())
	promptRepo := repository.NewPromptRepository(resources.DBConn())
	keywordRepo := repository.NewKeywordRepository(resources.DBConn())

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
	authService := authsvc.NewService(userRepo, verificationRepo, tokens, refreshStore, captchaManager, emailSender)
	authHandler := handler.NewAuthHandler(authService, limiter, verificationLimit, verificationWindow)

	userService := usersvc.NewService(userRepo, modelRepo)
	userHandler := handler.NewUserHandler(userService)

	modelService := modelsvc.NewService(modelRepo, userRepo)
	modelHandler := handler.NewModelHandler(modelService)
	promptService := promptsvc.NewService(promptRepo, keywordRepo, modelService, workspaceStore, persistenceQueue, logger)
	promptRateLimit := loadPromptRateLimit(logger)
	promptHandler := handler.NewPromptHandler(promptService, promptLimiter, promptRateLimit)
	if workspaceStore != nil && persistenceQueue != nil {
		promptService.StartPersistenceWorker(ctx, 0)
	}
	changelogService := changelogsrv.NewService(changelogRepo)
	changelogHandler := handler.NewChangelogHandler(changelogService, modelService)

	uploadStorage := filepath.Join("public", "avatars")
	uploadHandler := handler.NewUploadHandler(uploadStorage)

	authMiddleware := middleware.NewAuthMiddleware(cfg.JWTSecret)

	router := server.NewRouter(server.RouterOptions{
		AuthHandler:      authHandler,
		UserHandler:      userHandler,
		UploadHandler:    uploadHandler,
		ModelHandler:     modelHandler,
		ChangelogHandler: changelogHandler,
		PromptHandler:    promptHandler,
		AuthMW:           authMiddleware,
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
