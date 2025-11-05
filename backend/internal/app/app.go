/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 19:54:47
 * @FilePath: \electron-go-app\backend\internal\app\app.go
 * @LastEditTime: 2025-10-09 19:39:43
 */
package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	bootstrapdata "electron-go-app/backend/internal/bootstrapdata"
	"electron-go-app/backend/internal/config"
	adminmetricsdomain "electron-go-app/backend/internal/domain/adminmetrics"
	changelog "electron-go-app/backend/internal/domain/changelog"
	promptdomain "electron-go-app/backend/internal/domain/prompt"
	domain "electron-go-app/backend/internal/domain/user"
	infra "electron-go-app/backend/internal/infra/client"
	appLogger "electron-go-app/backend/internal/infra/logger"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AppConfig 描述应用启动所需的核心配置（MySQL、Redis 等）。
type AppConfig struct {
	Mode  string
	MySQL infra.MySQLConfig
	Redis *infra.RedisOptions
	Local config.LocalRuntime
}

// Resources 封装运行期共享资源，包括 ORM 实例与原始数据库连接。
type Resources struct {
	Config AppConfig
	DB     *gorm.DB
	rawDB  *sql.DB
	Redis  *redis.Client
}

// InitResources 负责读取远端配置、建立数据库/Redis 连接并执行模型迁移。
// 返回的 Resources 会交由业务层（`internal/bootstrap`）继续组装依赖。
func InitResources(ctx context.Context) (*Resources, error) {
	config.LoadEnvFiles()

	runtimeFlags := config.LoadRuntimeFlags()
	if strings.EqualFold(runtimeFlags.Mode, config.ModeLocal) {
		return initLocalResources(ctx, runtimeFlags)
	}

	mysqlCfg, err := infra.LoadMySQLConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("load mysql config: %w", err)
	}

	gormDB, rawDB, err := infra.NewGORMMySQL(mysqlCfg)
	if err != nil {
		return nil, fmt.Errorf("connect mysql: %w", err)
	}

	// 保证核心模型已经同步到数据库，避免后续查询报错。
	if err := gormDB.AutoMigrate(
		&domain.User{},
		&domain.EmailVerificationToken{},
		&domain.UserModelCredential{},
		&changelog.Entry{},
		&promptdomain.Prompt{},
		&promptdomain.Keyword{},
		&promptdomain.PromptKeyword{},
		&promptdomain.PromptLike{},
		&promptdomain.PromptVersion{},
		&promptdomain.PublicPrompt{},
		&promptdomain.PromptComment{},
		&promptdomain.PromptCommentLike{},
		&adminmetricsdomain.DailyRecord{},
		&adminmetricsdomain.EventRecord{},
	); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	var (
		redisOpts   *infra.RedisOptions
		redisClient *redis.Client
	)

	// 如果设置了 Redis 端点，则尝试建立连接，为验证码、限流、刷新令牌等功能提供存储。
	if endpoint := strings.TrimSpace(os.Getenv("REDIS_ENDPOINT")); endpoint != "" {
		opts, err := infra.NewDefaultRedisOptions()
		if err != nil {
			return nil, fmt.Errorf("load redis config: %w", err)
		}
		client, err := infra.NewRedisClient(opts)
		if err != nil {
			return nil, fmt.Errorf("connect redis: %w", err)
		}
		redisOpts = &opts
		redisClient = client
	} else {
		appLogger.S().Infow("redis not configured, captcha feature disabled")
	}

	return &Resources{
		Config: AppConfig{
			Mode:  runtimeFlags.Mode,
			MySQL: mysqlCfg,
			Redis: redisOpts,
			Local: runtimeFlags.Local,
		},
		DB:    gormDB,
		rawDB: rawDB,
		Redis: redisClient,
	}, nil
}

const (
	localConnMaxLifetime = time.Hour
	localMaxOpenConns    = 1
	localMaxIdleConns    = 1
)

// initLocalResources 针对本地模式初始化 SQLite 连接，并确保默认用户存在。
func initLocalResources(ctx context.Context, flags config.RuntimeFlags) (*Resources, error) {
	dbPath := flags.Local.DBPath
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite directory: %w", err)
	}

	gormDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("extract sqlite db: %w", err)
	}
	sqlDB.SetConnMaxLifetime(localConnMaxLifetime)
	sqlDB.SetMaxOpenConns(localMaxOpenConns)
	sqlDB.SetMaxIdleConns(localMaxIdleConns)

	if err := gormDB.AutoMigrate(
		&domain.User{},
		&domain.EmailVerificationToken{},
		&domain.UserModelCredential{},
		&changelog.Entry{},
		&promptdomain.Prompt{},
		&promptdomain.Keyword{},
		&promptdomain.PromptKeyword{},
		&promptdomain.PromptLike{},
		&promptdomain.PromptVersion{},
		&promptdomain.PublicPrompt{},
		&promptdomain.PromptComment{},
		&promptdomain.PromptCommentLike{},
		&adminmetricsdomain.DailyRecord{},
		&adminmetricsdomain.EventRecord{},
	); err != nil {
		return nil, fmt.Errorf("auto migrate sqlite: %w", err)
	}

	if _, err := ensureLocalUser(ctx, gormDB, flags.Local); err != nil {
		return nil, fmt.Errorf("ensure local user: %w", err)
	}

	var reviewerID *uint
	if flags.Local.IsAdmin {
		reviewerID = &flags.Local.UserID
	}

	if err := bootstrapdata.SeedLocalDatabase(ctx, gormDB, bootstrapdata.Options{
		AuthorUserID:   flags.Local.UserID,
		ReviewerUserID: reviewerID,
		Logger:         appLogger.S().With("component", "bootstrapdata"),
	}); err != nil {
		return nil, fmt.Errorf("seed local database: %w", err)
	}

	appLogger.S().Infow("local mode enabled", "db_path", dbPath, "user_id", flags.Local.UserID)

	return &Resources{
		Config: AppConfig{
			Mode:  config.ModeLocal,
			Local: flags.Local,
		},
		DB:    gormDB,
		rawDB: sqlDB,
		Redis: nil,
	}, nil
}

// ensureLocalUser 确保本地模式下存在默认用户，便于后续请求注入身份。
func ensureLocalUser(ctx context.Context, db *gorm.DB, local config.LocalRuntime) (*domain.User, error) {
	var user domain.User
	err := db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", local.UserID).
		First(&user).Error
	switch {
	case err == nil:
		needsUpdate := false
		currentUsername := strings.TrimSpace(user.Username)
		desiredUsername := strings.TrimSpace(local.Username)
		if desiredUsername != "" && currentUsername == "" {
			user.Username = desiredUsername
			needsUpdate = true
		}

		currentEmail := strings.TrimSpace(user.Email)
		desiredEmail := strings.TrimSpace(local.Email)
		if desiredEmail != "" && currentEmail == "" {
			user.Email = desiredEmail
			needsUpdate = true
		}

		if !user.IsAdmin {
			user.IsAdmin = true
			needsUpdate = true
		}
		if needsUpdate {
			if saveErr := db.WithContext(ctx).Save(&user).Error; saveErr != nil {
				return nil, saveErr
			}
		}
		return &user, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		settingsJSON, serr := domain.SettingsJSON(domain.DefaultSettings())
		if serr != nil {
			return nil, serr
		}
		now := time.Now()
		user = domain.User{
			ID:              local.UserID,
			Username:        local.Username,
			Email:           local.Email,
			PasswordHash:    "LOCAL_ONLY",
			IsAdmin:         true,
			Settings:        settingsJSON,
			EmailVerifiedAt: &now,
		}
		if createErr := db.WithContext(ctx).Create(&user).Error; createErr != nil {
			return nil, createErr
		}
		return &user, nil
	default:
		return nil, err
	}
}

// Close 释放底层数据库连接资源，供进程退出时调用。
func (r *Resources) Close() error {
	if r == nil {
		return nil
	}
	if r.rawDB != nil {
		if err := r.rawDB.Close(); err != nil {
			return err
		}
	}
	if r.Redis != nil {
		if err := r.Redis.Close(); err != nil {
			return err
		}
	}
	return nil
}

// DBConn 返回共享的 *gorm.DB，用于业务层构建仓储等依赖。
func (r *Resources) DBConn() *gorm.DB {
	if r == nil {
		return nil
	}
	return r.DB
}

// WithShutdown 包裹带取消能力的执行函数，统一处理错误与收尾逻辑。
func WithShutdown(ctx context.Context, cancel func(), fn func(context.Context) error) {
	defer cancel()
	if err := fn(ctx); err != nil {
		appLogger.S().Fatalw("application error", "error", err)
	}
}
