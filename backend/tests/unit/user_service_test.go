/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 22:46:44
 * @FilePath: \electron-go-app\backend\tests\unit\user_service_test.go
 * @LastEditTime: 2025-10-08 22:46:49
 */
package unit

import (
	"context"
	"errors"
	"testing"

	domain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/repository"
	usersvc "electron-go-app/backend/internal/service/user"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupUserService 使用内存 SQLite 构建用户仓储与服务，供单元测试隔离依赖。
func setupUserService(t *testing.T) (*usersvc.Service, *repository.UserRepository, *gorm.DB) {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&domain.User{}, &domain.UserModelCredential{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	userRepo := repository.NewUserRepository(db)
	modelRepo := repository.NewModelCredentialRepository(db)
	svc := usersvc.NewService(userRepo, modelRepo)
	return svc, userRepo, db
}

// TestUserService_GetProfile 断言服务能够返回用户资料并填充默认设置。
func TestUserService_GetProfile(t *testing.T) {
	svc, repo, _ := setupUserService(t)

	u := &domain.User{
		Username: "alice",
		Email:    "alice@example.com",
		Settings: "",
	}
	if err := repo.Create(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	profile, err := svc.GetProfile(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("GetProfile returned error: %v", err)
	}
	if profile.User.ID != u.ID {
		t.Fatalf("expected user id %d, got %d", u.ID, profile.User.ID)
	}
	if profile.Settings.PreferredModel != domain.DefaultSettings().PreferredModel {
		t.Fatalf("unexpected preferred model: %s", profile.Settings.PreferredModel)
	}
}

// TestUserService_UpdateSettings 验证更新设置后返回值与数据库落库一致。
func TestUserService_UpdateSettings(t *testing.T) {
	svc, repo, db := setupUserService(t)

	u := &domain.User{
		Username: "bob",
		Email:    "bob@example.com",
		Settings: "",
	}
	if err := repo.Create(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	credential := domain.UserModelCredential{
		UserID:      u.ID,
		Provider:    "openai",
		ModelKey:    "gpt-4o-mini",
		DisplayName: "OpenAI GPT-4o-mini",
		Status:      "enabled",
	}
	if err := db.Create(&credential).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	desired := domain.Settings{
		PreferredModel: "gpt-4o-mini",
		SyncEnabled:    true,
	}

	profile, err := svc.UpdateSettings(context.Background(), u.ID, desired)
	if err != nil {
		t.Fatalf("UpdateSettings returned error: %v", err)
	}
	if profile.Settings != desired {
		t.Fatalf("settings not updated: %+v", profile.Settings)
	}

	var stored domain.User
	if err := db.First(&stored, u.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	parsed, err := domain.ParseSettings(stored.Settings)
	if err != nil {
		t.Fatalf("parse stored settings: %v", err)
	}
	if parsed != desired {
		t.Fatalf("expected stored settings %+v, got %+v", desired, parsed)
	}
}

// TestUserService_UserNotFound 确认查询或更新不存在用户时返回 ErrUserNotFound。
func TestUserService_UserNotFound(t *testing.T) {
	svc, _, _ := setupUserService(t)

	if _, err := svc.GetProfile(context.Background(), 42); err != usersvc.ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}

	_, err := svc.UpdateSettings(context.Background(), 42, domain.DefaultSettings())
	if err != usersvc.ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound on update, got %v", err)
	}
}

// TestUserService_UpdateProfile 验证基础信息更新与冲突检测。
func TestUserService_UpdateProfile(t *testing.T) {
	svc, repo, db := setupUserService(t)

	userA := &domain.User{Username: "alice", Email: "alice@example.com"}
	if err := repo.Create(context.Background(), userA); err != nil {
		t.Fatalf("create userA: %v", err)
	}
	userB := &domain.User{Username: "bob", Email: "bob@example.com"}
	if err := repo.Create(context.Background(), userB); err != nil {
		t.Fatalf("create userB: %v", err)
	}

	newUsername := "alice_new"
	newEmail := "alice_new@example.com"
	avatar := "https://example.com/avatar.png"

	profile, err := svc.UpdateProfile(context.Background(), userA.ID, usersvc.UpdateProfileParams{
		Username:  &newUsername,
		Email:     &newEmail,
		AvatarURL: &avatar,
	})
	if err != nil {
		t.Fatalf("UpdateProfile returned error: %v", err)
	}
	if profile.User.Username != newUsername {
		t.Fatalf("expected username updated, got %s", profile.User.Username)
	}
	if profile.User.Email != newEmail {
		t.Fatalf("expected email updated, got %s", profile.User.Email)
	}
	if profile.User.AvatarURL != avatar {
		t.Fatalf("expected avatar updated, got %s", profile.User.AvatarURL)
	}

	var stored domain.User
	if err := db.First(&stored, userA.ID).Error; err != nil {
		t.Fatalf("reload userA: %v", err)
	}
	if stored.Username != newUsername || stored.Email != newEmail || stored.AvatarURL != avatar {
		t.Fatalf("stored user mismatch: %+v", stored)
	}

	dupUsername := "bob"
	if _, err := svc.UpdateProfile(context.Background(), userA.ID, usersvc.UpdateProfileParams{Username: &dupUsername}); err != usersvc.ErrUsernameTaken {
		t.Fatalf("expected ErrUsernameTaken, got %v", err)
	}

	dupEmail := "bob@example.com"
	if _, err := svc.UpdateProfile(context.Background(), userA.ID, usersvc.UpdateProfileParams{Email: &dupEmail}); err != usersvc.ErrEmailTaken {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
}

func TestUserService_UpdateSettings_ValidatePreferredModel(t *testing.T) {
	svc, repo, db := setupUserService(t)

	user := &domain.User{Username: "alice", Email: "alice@example.com"}
	if err := repo.Create(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	cred := domain.UserModelCredential{
		UserID:      user.ID,
		Provider:    "openai",
		ModelKey:    "primary",
		DisplayName: "OpenAI",
		Status:      "enabled",
	}
	if err := db.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	settings := domain.Settings{PreferredModel: "primary", SyncEnabled: true}
	if _, err := svc.UpdateSettings(context.Background(), user.ID, settings); err != nil {
		t.Fatalf("UpdateSettings returned error: %v", err)
	}

	status := "disabled"
	cred.Status = status
	if err := db.Save(&cred).Error; err != nil {
		t.Fatalf("disable credential: %v", err)
	}

	settings.PreferredModel = "primary"
	if _, err := svc.UpdateSettings(context.Background(), user.ID, settings); !errors.Is(err, usersvc.ErrPreferredModelDisabled) {
		t.Fatalf("expected ErrPreferredModelDisabled, got %v", err)
	}

	settings.PreferredModel = "unknown"
	if _, err := svc.UpdateSettings(context.Background(), user.ID, settings); !errors.Is(err, usersvc.ErrPreferredModelNotFound) {
		t.Fatalf("expected ErrPreferredModelNotFound, got %v", err)
	}

	cred.Status = "enabled"
	if err := db.Save(&cred).Error; err != nil {
		t.Fatalf("restore credential: %v", err)
	}
	settings.PreferredModel = "primary"
	if _, err := svc.UpdateSettings(context.Background(), user.ID, settings); err != nil {
		t.Fatalf("expected success for enabled model, got %v", err)
	}

	settings.PreferredModel = domain.DefaultSettings().PreferredModel
	if _, err := svc.UpdateSettings(context.Background(), user.ID, settings); err != nil {
		t.Fatalf("expected success for default model, got %v", err)
	}
}

func TestUserService_UpdateSettings_EmptyPreferredModelResetsToDefault(t *testing.T) {
	svc, repo, db := setupUserService(t)

	user := &domain.User{Username: "carol", Email: "carol@example.com"}
	if err := repo.Create(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	cred := domain.UserModelCredential{
		UserID:      user.ID,
		Provider:    "openai",
		ModelKey:    "primary",
		DisplayName: "OpenAI",
		Status:      "enabled",
	}
	if err := db.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	initial := domain.Settings{PreferredModel: "primary", SyncEnabled: true}
	if _, err := svc.UpdateSettings(context.Background(), user.ID, initial); err != nil {
		t.Fatalf("set initial preferred model: %v", err)
	}

	cleared := domain.Settings{PreferredModel: "", SyncEnabled: false}
	profile, err := svc.UpdateSettings(context.Background(), user.ID, cleared)
	if err != nil {
		t.Fatalf("update with empty preferred model: %v", err)
	}

	if profile.Settings.PreferredModel != domain.DefaultSettings().PreferredModel {
		t.Fatalf("expected default preferred model, got %s", profile.Settings.PreferredModel)
	}
}
