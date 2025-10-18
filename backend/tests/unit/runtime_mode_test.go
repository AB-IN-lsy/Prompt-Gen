package unit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"electron-go-app/backend/internal/app"
	"electron-go-app/backend/internal/config"
	domain "electron-go-app/backend/internal/domain/user"
)

// TestInitResourcesLocalMode 验证 APP_MODE=local 时服务会切换到 SQLite 并创建默认用户。
func TestInitResourcesLocalMode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()
	sqlitePath := filepath.Join(tmpDir, "promptgen-local.db")
	const (
		customUsername = "测试离线用户"
		customEmail    = "local@example.com"
	)

	config.SetEnvFileLoadingForTest(false)
	t.Cleanup(func() {
		config.SetEnvFileLoadingForTest(true)
	})

	setenv(t, "APP_MODE", config.ModeLocal)
	setenv(t, "LOCAL_SQLITE_PATH", sqlitePath)
	setenv(t, "LOCAL_USER_ID", "99")
	setenv(t, "LOCAL_USER_USERNAME", customUsername)
	setenv(t, "LOCAL_USER_EMAIL", customEmail)
	setenv(t, "LOCAL_USER_ADMIN", "true")

	resources, err := app.InitResources(ctx)
	if err != nil {
		t.Fatalf("InitResources failed: %v", err)
	}
	t.Cleanup(func() {
		if err := resources.Close(); err != nil {
			t.Fatalf("close resources: %v", err)
		}
	})

	if resources.Config.Mode != config.ModeLocal {
		t.Fatalf("expected mode %q, got %q", config.ModeLocal, resources.Config.Mode)
	}
	if name := resources.DB.Dialector.Name(); name != "sqlite" {
		t.Fatalf("expected sqlite dialector, got %s", name)
	}
	if resources.Config.Local.DBPath != sqlitePath {
		t.Fatalf("unexpected db path: want %s got %s", sqlitePath, resources.Config.Local.DBPath)
	}

	var user domain.User
	if err := resources.DB.WithContext(ctx).First(&user, resources.Config.Local.UserID).Error; err != nil {
		t.Fatalf("query default user: %v", err)
	}
	if user.Username != customUsername {
		t.Fatalf("username mismatch: want %s got %s", customUsername, user.Username)
	}
	if user.Email != customEmail {
		t.Fatalf("email mismatch: want %s got %s", customEmail, user.Email)
	}
	if !user.IsAdmin {
		t.Fatalf("expected user to be admin")
	}
}

func setenv(t *testing.T, key, value string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("setenv %s=%s failed: %v", key, value, err)
	}
	t.Cleanup(func() {
		if !existed {
			_ = os.Unsetenv(key)
			return
		}
		_ = os.Setenv(key, old)
	})
}
