/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 19:54:47
 * @FilePath: \electron-go-app\backend\internal\app\app.go
 * @LastEditTime: 2025-10-08 19:54:51
 */
package app

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"electron-go-app/backend/internal/config"
	domain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/infra"
	appLogger "electron-go-app/backend/internal/infra/logger"

	"gorm.io/gorm"
)

// AppConfig 描述应用启动所需的核心配置，如 Nacos 与 MySQL。
type AppConfig struct {
	Nacos infra.NacosOptions
	MySQL infra.MySQLConfig
}

// Resources 封装运行期共享资源，包括 ORM 实例与原始数据库连接。
type Resources struct {
	Config AppConfig
	DB     *gorm.DB
	rawDB  *sql.DB
}

// Bootstrap 负责初始化配置、建立数据库连接并执行模型迁移。
// 返回的 Resources 会被上层用于依赖注入和生命周期管理。
func Bootstrap(ctx context.Context) (*Resources, error) {
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

	if err := gormDB.AutoMigrate(&domain.User{}); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	return &Resources{
		Config: AppConfig{
			Nacos: nacosOpts,
			MySQL: mysqlCfg,
		},
		DB:    gormDB,
		rawDB: rawDB,
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
