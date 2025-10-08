/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 17:59:47
 * @FilePath: \electron-go-app\backend\tests\unit\infra_nacos_test.go
 * @LastEditTime: 2025-10-08 17:59:52
 */
package unit

import (
	"os"
	"testing"

	"electron-go-app/backend/internal/config"
	"electron-go-app/backend/internal/infra"
)

func TestNewDefaultNacosOptions_RequiresEnv(t *testing.T) {
	config.SetEnvFileLoadingForTest(false)
	t.Cleanup(func() { config.SetEnvFileLoadingForTest(true) })

	os.Unsetenv("NACOS_ENDPOINT")
	os.Unsetenv("NACOS_USERNAME")
	os.Unsetenv("NACOS_PASSWORD")

	if _, err := infra.NewDefaultNacosOptions(); err == nil {
		t.Fatalf("expected error when env vars missing")
	}
}

func TestNewDefaultNacosOptions_LoadsValues(t *testing.T) {
	config.SetEnvFileLoadingForTest(false)
	t.Cleanup(func() { config.SetEnvFileLoadingForTest(true) })

	t.Setenv("NACOS_ENDPOINT", "127.0.0.1:8848")
	t.Setenv("NACOS_USERNAME", "nacos")
	t.Setenv("NACOS_PASSWORD", "secret")
	t.Setenv("NACOS_GROUP", "CUSTOM")
	t.Setenv("NACOS_NAMESPACE", "ns")
	t.Setenv("NACOS_CONTEXT_PATH", "/ctx")

	opts, err := infra.NewDefaultNacosOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.Host != "127.0.0.1" || opts.Port != 8848 {
		t.Fatalf("unexpected host/port: %s:%d", opts.Host, opts.Port)
	}

	if opts.Group != "CUSTOM" {
		t.Fatalf("expected group CUSTOM, got %s", opts.Group)
	}

	if opts.NamespaceID != "ns" {
		t.Fatalf("expected namespace ns, got %s", opts.NamespaceID)
	}

	if opts.ContextPath != "/ctx" {
		t.Fatalf("expected context path /ctx, got %s", opts.ContextPath)
	}

	if opts.Username != "nacos" || opts.Password != "secret" {
		t.Fatalf("expected credentials to match provided env")
	}
}
