/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 18:00:02
 * @FilePath: \electron-go-app\backend\tests\unit\infra_mysql_test.go
 * @LastEditTime: 2025-10-09 19:37:07
 */
package unit

import (
	"testing"

	infra "electron-go-app/backend/internal/infra/client"
)

// TestParseMySQLConfigDefaults 验证解析配置时能补齐默认端口、数据库与参数。
func TestParseMySQLConfigDefaults(t *testing.T) {
	data := []byte(`{
        "mysql.host": "127.0.0.1",
        "mysql.username": "root",
        "mysql.password": "secret"
    }`)

	cfg, err := infra.ParseMySQLConfig(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 3306 {
		t.Fatalf("expected default port 3306, got %d", cfg.Port)
	}

	if cfg.Database != "prompt" {
		t.Fatalf("expected default database prompt, got %s", cfg.Database)
	}

	if cfg.Params == "" {
		t.Fatalf("expected params to default")
	}
}

// TestBuildMySQLDSN 构造完整 DSN，确保字段映射无误。
func TestBuildMySQLDSN(t *testing.T) {
	cfg := infra.MySQLConfig{
		Host:     "127.0.0.1",
		Port:     3310,
		Username: "root",
		Password: "secret",
		Database: "prompt",
		Params:   "charset=utf8mb4",
	}

	dsn, err := infra.BuildMySQLDSN(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "root:secret@tcp(127.0.0.1:3310)/prompt?charset=utf8mb4"
	if dsn != expected {
		t.Fatalf("expected %s, got %s", expected, dsn)
	}
}

// TestBuildMySQLDSNValidation 确认缺失必要字段时返回验证错误。
func TestBuildMySQLDSNValidation(t *testing.T) {
	_, err := infra.BuildMySQLDSN(infra.MySQLConfig{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}
