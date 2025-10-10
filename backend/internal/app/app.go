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
	"fmt"
	"os"
	"strings"

	"electron-go-app/backend/internal/config"
	domain "electron-go-app/backend/internal/domain/user"
	infra "electron-go-app/backend/internal/infra/client"
	appLogger "electron-go-app/backend/internal/infra/logger"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// AppConfig 描述应用启动所需的核心配置，如 Nacos 与 MySQL。
type AppConfig struct {
	Nacos infra.NacosOptions
	MySQL infra.MySQLConfig
	Redis *infra.RedisOptions
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

	nacosOpts, err := infra.NewDefaultNacosOptions()
	if err != nil {
		return nil, fmt.Errorf("build nacos options: %w", err)
	}

	group := os.Getenv("MYSQL_CONFIG_GROUP")
	if group == "" {
		group = "DEFAULT_GROUP"
	}

	mysqlCfg, err := infra.LoadMySQLConfig(ctx, nacosOpts, group)
	if err != nil {
		return nil, fmt.Errorf("load mysql config: %w", err)
	}

	gormDB, rawDB, err := infra.NewGORMMySQL(mysqlCfg)
	if err != nil {
		return nil, fmt.Errorf("connect mysql: %w", err)
	}

	// 保证核心模型已经同步到数据库，避免后续查询报错。
	if err := gormDB.AutoMigrate(&domain.User{}, &domain.EmailVerificationToken{}, &domain.UserModelCredential{}); err != nil {
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
			Nacos: nacosOpts,
			MySQL: mysqlCfg,
			Redis: redisOpts,
		},
		DB:    gormDB,
		rawDB: rawDB,
		Redis: redisClient,
	}, nil
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
