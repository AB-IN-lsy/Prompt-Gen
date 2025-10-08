//go:build integration

/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 18:00:20
 * @FilePath: \electron-go-app\backend\tests\integration\nacos_integration_test.go
 * @LastEditTime: 2025-10-08 18:00:26
 */

package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"electron-go-app/backend/internal/infra"
)

func TestNacosGetConfig(t *testing.T) {
	// 集成测试场景：向真实或准生产 Nacos 拉配置，验证账号、网络与注册信息是否正确。
	// 需显式设置 INTEGRATION=1 才会运行，避免本地/CI 在缺少依赖时误触发。
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("integration tests disabled")
	}

	opts, err := infra.NewDefaultNacosOptions()
	if err != nil {
		t.Fatalf("failed to build options: %v", err)
	}

	// 通过环境变量指定要拉取的配置项，方便针对不同环境（测试、预发、生产）切换。
	dataID := os.Getenv("NACOS_TEST_DATA_ID")
	if dataID == "" {
		t.Skip("NACOS_TEST_DATA_ID not set")
	}

	group := os.Getenv("NACOS_TEST_GROUP")
	if group == "" {
		group = "DEFAULT_GROUP"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	content, err := infra.FetchNacosConfig(ctx, opts, dataID, group)
	if err != nil {
		t.Fatalf("failed to fetch nacos config: %v", err)
	}

	// 配置为空通常意味着 Nacos 数据缺失或权限问题，直接报错提示。
	if strings.TrimSpace(content) == "" {
		t.Fatalf("nacos config is empty")
	}
}
