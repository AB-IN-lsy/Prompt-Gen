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

// MySQLConfig holds database connection details resolved from Nacos.
type MySQLConfig struct {
	Host     string `json:"mysql.host"`
	Port     int    `json:"mysql.port"`
	Username string `json:"mysql.username"`
	Password string `json:"mysql.password"`
	Database string `json:"mysql.database"`
	Params   string `json:"mysql.params"`
}

// LoadMySQLConfig pulls and parses the MySQL configuration from Nacos.
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

// NewMySQLConn opens a *sql.DB using the provided configuration.
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

// ParseMySQLConfig parses JSON configuration content, supplying defaults for missing optional fields.
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

// BuildMySQLDSN renders a DSN string for the provided configuration after validation.
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
