/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 21:21:28
 * @FilePath: \electron-go-app\backend\tests\unit\auth_service_test.go
 * @LastEditTime: 2025-10-08 21:21:32
 */
package unit

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	domain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/infra/token"
	"electron-go-app/backend/internal/repository"
	auth "electron-go-app/backend/internal/service/auth"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestAuthService(t *testing.T) (*auth.Service, *repository.UserRepository, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&domain.User{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := repository.NewUserRepository(db)
	tokenManager := token.NewJWTManager("test-secret", time.Minute, 24*time.Hour)
	service := auth.NewService(repo, tokenManager)

	return service, repo, db
}

func TestAuthServiceRegisterAndLogin(t *testing.T) {
	svc, repo, _ := newTestAuthService(t)
	ctx := context.Background()

	user, tokens, err := svc.Register(ctx, auth.RegisterParams{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if user.ID == 0 {
		t.Fatalf("expected persisted user ID")
	}

	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("expected non-empty tokens")
	}

	if tokens.ExpiresIn <= 0 {
		t.Fatalf("expected positive access token ttl, got %d", tokens.ExpiresIn)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("password123")); err != nil {
		t.Fatalf("password not hashed correctly: %v", err)
	}

	loginUser, loginTokens, err := svc.Login(ctx, auth.LoginParams{
		Email:    "alice@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if loginUser.LastLoginAt == nil {
		t.Fatalf("expected last login timestamp to be set")
	}

	if loginTokens.AccessToken == "" || loginTokens.RefreshToken == "" {
		t.Fatalf("expected login tokens")
	}

	stored, err := repo.FindByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	if stored.LastLoginAt == nil {
		t.Fatalf("expected stored user to contain last login timestamp")
	}
}

func TestAuthServiceRegisterDuplicateEmailAndUsername(t *testing.T) {
	svc, _, _ := newTestAuthService(t)
	ctx := context.Background()

	_, _, err := svc.Register(ctx, auth.RegisterParams{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("register first user: %v", err)
	}

	_, _, err = svc.Register(ctx, auth.RegisterParams{
		Username: "bob",
		Email:    "alice@example.com",
		Password: "password123",
	})
	if !errors.Is(err, auth.ErrEmailTaken) {
		t.Fatalf("expected email taken error, got %v", err)
	}

	_, _, err = svc.Register(ctx, auth.RegisterParams{
		Username: "alice",
		Email:    "alice2@example.com",
		Password: "password123",
	})
	if !errors.Is(err, auth.ErrUsernameTaken) {
		t.Fatalf("expected username taken error, got %v", err)
	}
}

func TestAuthServiceLoginInvalidCredentials(t *testing.T) {
	svc, _, _ := newTestAuthService(t)
	ctx := context.Background()

	_, _, err := svc.Register(ctx, auth.RegisterParams{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	_, _, err = svc.Login(ctx, auth.LoginParams{
		Email:    "alice@example.com",
		Password: "wrong-password",
	})
	if !errors.Is(err, auth.ErrInvalidLogin) {
		t.Fatalf("expected invalid login error, got %v", err)
	}

	_, _, err = svc.Login(ctx, auth.LoginParams{
		Email:    "unknown@example.com",
		Password: "password123",
	})
	if !errors.Is(err, auth.ErrInvalidLogin) {
		t.Fatalf("expected invalid login for unknown user, got %v", err)
	}
}
