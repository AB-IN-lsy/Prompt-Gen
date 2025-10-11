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
	"time"

	"electron-go-app/backend/internal/app"
	"electron-go-app/backend/internal/handler"
	"electron-go-app/backend/internal/infra/captcha"
	"electron-go-app/backend/internal/infra/email"
	"electron-go-app/backend/internal/infra/ratelimit"
	"electron-go-app/backend/internal/infra/token"
	"electron-go-app/backend/internal/middleware"
	"electron-go-app/backend/internal/repository"
	"electron-go-app/backend/internal/server"
	authsvc "electron-go-app/backend/internal/service/auth"
	changelogsrv "electron-go-app/backend/internal/service/changelog"
	modelsvc "electron-go-app/backend/internal/service/model"
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

	// 刷新令牌优先落在 Redis，便于服务重启后继续验证。
	var refreshStore authsvc.RefreshTokenStore
	if resources.Redis != nil {
		refreshStore = token.NewRedisRefreshTokenStore(resources.Redis, "")
	} else {
		refreshStore = token.NewMemoryRefreshTokenStore()
		logger.Infow("using in-memory refresh token store; tokens won't persist across restarts")
	}

	// 验证码管理器是可选的：未启用时返回 nil，后续 Handler 会自检。
	captchaManager, err := initCaptchaManager(resources, logger)
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

	// 支持通过环境变量自定义验证邮件的频率限制。
	verificationLimit, verificationWindow := loadEmailVerificationRateConfig(logger)

	// 汇总依赖，构建鉴权服务与 Handler。
	authService := authsvc.NewService(userRepo, verificationRepo, tokens, refreshStore, captchaManager, emailSender)
	authHandler := handler.NewAuthHandler(authService, limiter, verificationLimit, verificationWindow)

	userService := usersvc.NewService(userRepo, modelRepo)
	userHandler := handler.NewUserHandler(userService)

	modelService := modelsvc.NewService(modelRepo, userRepo)
	modelHandler := handler.NewModelHandler(modelService)
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
		AuthMW:           authMiddleware,
	})

	return &Application{
		Resources: resources,
		AuthSvc:   authService,
		UserSvc:   userService,
		ModelSvc:  modelService,
		Changelog: changelogService,
		Router:    router,
	}, nil
}

func initCaptchaManager(resources *app.Resources, logger *zap.SugaredLogger) (authsvc.CaptchaManager, error) {
	// 从环境变量解析验证码图像、限流等配置。
	captchaOpts, captchaEnabled, err := captcha.LoadOptionsFromEnv()
	if err != nil {
		logger.Errorw("load captcha config failed", "error", err)
		return nil, fmt.Errorf("load captcha config: %w", err)
	}

	if !captchaEnabled {
		return nil, nil
	}

	if resources.Redis == nil {
		return nil, fmt.Errorf("captcha enabled but redis not configured")
	}

	// 为验证码生成独立限流前缀，防止与邮箱验证冲突。
	limiter := ratelimit.NewRedisLimiter(resources.Redis, fmt.Sprintf("%s_rl", captchaOpts.Prefix))
	manager := captcha.NewManager(resources.Redis, limiter, captchaOpts)
	logger.Infow("captcha enabled", "prefix", captchaOpts.Prefix, "ttl", captchaOpts.TTL)
	return manager, nil
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
