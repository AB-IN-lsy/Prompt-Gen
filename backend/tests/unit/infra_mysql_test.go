/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 18:00:02
 * @FilePath: \electron-go-app\backend\tests\unit\infra_mysql_test.go
 * @LastEditTime: 2025-10-08 18:00:07
 */
package unit

import (
	"testing"

	"electron-go-app/backend/internal/infra"
)

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

func TestBuildMySQLDSNValidation(t *testing.T) {
	_, err := infra.BuildMySQLDSN(infra.MySQLConfig{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}
