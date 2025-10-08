/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 22:46:44
 * @FilePath: \electron-go-app\backend\tests\unit\user_service_test.go
 * @LastEditTime: 2025-10-08 22:46:49
 */
package unit

import (
	"context"
	"testing"

	domain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/repository"
	usersvc "electron-go-app/backend/internal/service/user"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupUserService(t *testing.T) (*usersvc.Service, *repository.UserRepository, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&domain.User{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	repo := repository.NewUserRepository(db)
	svc := usersvc.NewService(repo)
	return svc, repo, db
}

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
