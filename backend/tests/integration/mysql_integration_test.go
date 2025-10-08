//go:build integration

/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 18:00:37
 * @FilePath: \electron-go-app\backend\tests\integration\mysql_integration_test.go
 * @LastEditTime: 2025-10-08 18:00:42
 */

package integration

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"electron-go-app/backend/internal/infra"
)

func TestMySQLPing(t *testing.T) {
	// 集成测试场景：对真实或准生产 MySQL 进行一次连接握手验证，确保配置链路可用。
	// 仅当设置 INTEGRATION=1 时才执行，以避免本地开发误连线上。
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("integration tests disabled")
	}

	// 优先从环境变量读取直连配置，方便在 CI 或生产环境下自定义目标库；
	// 若未提供，则回退到 Nacos 拉取配置，模拟线上真实流程。
	cfg, ok := mysqlConfigFromEnv()
	if !ok {
		opts, err := infra.NewDefaultNacosOptions()
		if err != nil {
			t.Fatalf("build nacos options: %v", err)
		}

		group := os.Getenv("MYSQL_CONFIG_GROUP")
		if group == "" {
			group = "DEFAULT_GROUP"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		cfg, err = infra.LoadMySQLConfig(ctx, opts, group)
		if err != nil {
			t.Fatalf("load mysql config from nacos: %v", err)
		}
	}

	// 只要能成功建立连接，说明凭证/网络/中间件等环节都正常，否则直接失败提示。
	db, err := infra.NewMySQLConn(cfg)
	if err != nil {
		t.Fatalf("failed to connect mysql: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
}

func mysqlConfigFromEnv() (infra.MySQLConfig, bool) {
	host := os.Getenv("MYSQL_TEST_HOST")
	user := os.Getenv("MYSQL_TEST_USER")
	pass := os.Getenv("MYSQL_TEST_PASS")
	dbName := os.Getenv("MYSQL_TEST_DB")

	if host == "" || user == "" || pass == "" || dbName == "" {
		return infra.MySQLConfig{}, false
	}

	cfg := infra.MySQLConfig{
		Host:     host,
		Username: user,
		Password: pass,
		Database: dbName,
		Params:   os.Getenv("MYSQL_TEST_PARAMS"),
	}

	if strings.Contains(host, ":") {
		h, p, err := net.SplitHostPort(host)
		if err == nil {
			cfg.Host = h
			if port, perr := strconv.Atoi(p); perr == nil {
				cfg.Port = port
			}
		}
	}

	return cfg, true
}
