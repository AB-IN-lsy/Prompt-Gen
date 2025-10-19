package unit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	backendapp "electron-go-app/backend/internal/app"
	"electron-go-app/backend/internal/config"
	userdomain "electron-go-app/backend/internal/domain/user"
)

const (
	localModeTestUsername = "离线测试用户"
	localModeTestEmail    = "offline@test.local"
)

// TestInitResourcesLocalModeDefaultAdmin 验证本地模式下初始化出的默认账号具备管理员权限。
func TestInitResourcesLocalModeDefaultAdmin(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "promptgen-local.db")

	config.SetEnvFileLoadingForTest(false)
	t.Cleanup(func() {
		config.SetEnvFileLoadingForTest(true)
	})

	setenv(t, "APP_MODE", config.ModeLocal)
	setenv(t, "LOCAL_SQLITE_PATH", dbPath)
	setenv(t, "LOCAL_USER_USERNAME", localModeTestUsername)
	setenv(t, "LOCAL_USER_EMAIL", localModeTestEmail)
	setenv(t, "LOCAL_USER_ADMIN", "true")

	resources, err := backendapp.InitResources(ctx)
	if err != nil {
		t.Fatalf("InitResources(local): %v", err)
	}
	t.Cleanup(func() {
		if err := resources.Close(); err != nil {
			t.Fatalf("cleanup resources: %v", err)
		}
	})

	if resources.Config.Mode != config.ModeLocal {
		t.Fatalf("expected mode=local, got=%s", resources.Config.Mode)
	}
	if resources.Redis != nil {
		t.Fatalf("expected redis to be nil in local mode")
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("sqlite db not created: %v", err)
	}

	var user userdomain.User
	if err := resources.DB.WithContext(ctx).First(&user, resources.Config.Local.UserID).Error; err != nil {
		t.Fatalf("load local user: %v", err)
	}
	if user.Username != localModeTestUsername {
		t.Fatalf("unexpected username: %s", user.Username)
	}
	if user.Email != localModeTestEmail {
		t.Fatalf("unexpected email: %s", user.Email)
	}
	if !user.IsAdmin {
		t.Fatalf("local admin flag not applied")
	}
}
