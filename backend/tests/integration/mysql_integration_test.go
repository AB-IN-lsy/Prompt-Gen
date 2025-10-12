//go:build integration

/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 18:00:37
 * @FilePath: \electron-go-app\backend\tests\integration\mysql_integration_test.go
 * @LastEditTime: 2025-10-08 18:00:42
 */

package integration

import (
	"os"
	"testing"

	infra "electron-go-app/backend/internal/infra/client"
)

func TestMySQLPing(t *testing.T) {
	// 集成测试场景：对真实或准生产 MySQL 进行一次连接握手验证，确保配置链路可用。
	// 仅当设置 INTEGRATION=1 时才执行，以避免本地开发误连线上。
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("integration tests disabled")
	}

	cfg, err := infra.LoadMySQLConfigFromEnv()
	if err != nil {
		t.Skipf("mysql config not provided: %v", err)
	}

	// 只要能成功建立连接，说明凭证/网络/中间件等环节都正常，否则直接失败提示。
	db, err := infra.NewMySQLConn(cfg)
	if err != nil {
		t.Fatalf("failed to connect mysql: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
}
