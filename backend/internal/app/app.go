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
	"log"
	"os"

	"electron-go-app/backend/internal/config"
	"electron-go-app/backend/internal/infra"
)

type AppConfig struct {
	Nacos infra.NacosOptions
	MySQL infra.MySQLConfig
}

type Resources struct {
	Config AppConfig
	DB     *sql.DB
}

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

	db, err := infra.NewMySQLConn(mysqlCfg)
	if err != nil {
		return nil, fmt.Errorf("connect mysql: %w", err)
	}

	return &Resources{
		Config: AppConfig{
			Nacos: nacosOpts,
			MySQL: mysqlCfg,
		},
		DB: db,
	}, nil
}

func (r *Resources) Close() error {
	if r == nil {
		return nil
	}
	if r.DB != nil {
		if err := r.DB.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resources) DBConn() *sql.DB {
	if r == nil {
		return nil
	}
	return r.DB
}

func WithShutdown(ctx context.Context, cancel func(), fn func(context.Context) error) {
	defer cancel()
	if err := fn(ctx); err != nil {
		log.Fatalf("application error: %v", err)
	}
}
