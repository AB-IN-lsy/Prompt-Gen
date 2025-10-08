/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 17:16:56
 * @FilePath: \electron-go-app\backend\internal\infra\mysql_client.go
 * @LastEditTime: 2025-10-08 19:49:45
 */
package infra

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"electron-go-app/backend/internal/config"

	_ "github.com/go-sql-driver/mysql"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	mysqlConfigDataIDEnv = "MYSQL_CONFIG_DATA_ID"
)

const (
	defaultMySQLDataID   = "mysql-config.properties"
	defaultMySQLPort     = 3306
	defaultMySQLDatabase = "prompt"
	defaultMySQLParams   = "charset=utf8mb4&parseTime=true&loc=Local"
)

// MySQLConfig 描述从 Nacos 下发的数据库连接配置项。
type MySQLConfig struct {
	Host     string `json:"mysql.host"`
	Port     int    `json:"mysql.port"`
	Username string `json:"mysql.username"`
	Password string `json:"mysql.password"`
	Database string `json:"mysql.database"`
	Params   string `json:"mysql.params"`
}

// LoadMySQLConfig 从 Nacos 拉取指定 group 的 MySQL 配置并解析。
func LoadMySQLConfig(ctx context.Context, opts NacosOptions, group string) (MySQLConfig, error) {
	config.LoadEnvFiles()

	dataID := os.Getenv(mysqlConfigDataIDEnv)
	if dataID == "" {
		dataID = defaultMySQLDataID
	}

	content, err := FetchNacosConfig(ctx, opts, dataID, group)
	if err != nil {
		return MySQLConfig{}, err
	}

	cfg, err := ParseMySQLConfig([]byte(content))
	if err != nil {
		return MySQLConfig{}, err
	}

	return cfg, nil
}

// NewMySQLConn 基于配置创建标准库 *sql.DB 连接。
func NewMySQLConn(cfg MySQLConfig) (*sql.DB, error) {
	dsn, err := BuildMySQLDSN(cfg)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	db.SetConnMaxLifetime(60 * time.Minute)
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(25)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	return db, nil
}

// NewGORMMySQL 创建 GORM 连接并返回 ORM 与底层 *sql.DB，便于控制生命周期。
func NewGORMMySQL(cfg MySQLConfig) (*gorm.DB, *sql.DB, error) {
	dsn, err := BuildMySQLDSN(cfg)
	if err != nil {
		return nil, nil, err
	}

	gormDB, err := gorm.Open(mysqlDriver.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, nil, fmt.Errorf("open gorm mysql: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, nil, fmt.Errorf("get sql db: %w", err)
	}

	sqlDB.SetConnMaxLifetime(60 * time.Minute)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(25)

	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("ping mysql: %w", err)
	}

	return gormDB, sqlDB, nil
}

// validateMySQLConfig 校验配置字段是否完整。
func validateMySQLConfig(cfg MySQLConfig) error {
	if cfg.Host == "" {
		return fmt.Errorf("mysql host is required")
	}
	if cfg.Username == "" {
		return fmt.Errorf("mysql username is required")
	}
	if cfg.Password == "" {
		return fmt.Errorf("mysql password is required")
	}
	if cfg.Database == "" {
		return fmt.Errorf("mysql database is required")
	}
	return nil
}

// ParseMySQLConfig 解析 JSON 文本并填充可选字段的默认值。
func ParseMySQLConfig(data []byte) (MySQLConfig, error) {
	var cfg MySQLConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return MySQLConfig{}, fmt.Errorf("parse mysql config: %w", err)
	}

	if cfg.Port == 0 {
		cfg.Port = defaultMySQLPort
	}

	if cfg.Database == "" {
		cfg.Database = defaultMySQLDatabase
	}

	if cfg.Params == "" {
		cfg.Params = defaultMySQLParams
	}

	return cfg, nil
}

// BuildMySQLDSN 在通过校验后拼接 MySQL DSN 字符串。
func BuildMySQLDSN(cfg MySQLConfig) (string, error) {
	if err := validateMySQLConfig(cfg); err != nil {
		return "", err
	}

	params := cfg.Params
	if params == "" {
		params = defaultMySQLParams
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		params,
	)

	return dsn, nil
}
