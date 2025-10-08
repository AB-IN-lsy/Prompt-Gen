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
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("integration tests disabled")
	}

	opts, err := infra.NewDefaultNacosOptions()
	if err != nil {
		t.Fatalf("failed to build options: %v", err)
	}

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

	if strings.TrimSpace(content) == "" {
		t.Fatalf("nacos config is empty")
	}
}
