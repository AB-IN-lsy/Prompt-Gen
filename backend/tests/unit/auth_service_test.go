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
	"electron-go-app/backend/internal/infra/captcha"
	"electron-go-app/backend/internal/infra/token"
	"electron-go-app/backend/internal/repository"
	auth "electron-go-app/backend/internal/service/auth"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type testEmailSender struct{}

func (testEmailSender) SendVerification(_ context.Context, _ *domain.User, _ string) error {
	return nil
}

type fakeCaptcha struct {
	expectedID   string
	expectedCode string
	returnErr    error
	verifyCalls  int
}

func (f *fakeCaptcha) Generate(_ context.Context, _ string) (string, string, int, error) {
	return "stub", "", -1, nil
}

func (f *fakeCaptcha) Verify(_ context.Context, id string, answer string) error {
	f.verifyCalls++
	if f.returnErr != nil {
		return f.returnErr
	}
	if id != f.expectedID || answer != f.expectedCode {
		return errors.New("unexpected captcha payload")
	}
	return nil
}

// newTestAuthService 创建内存版鉴权服务和仓储，便于单元测试隔离数据库依赖。
func newTestAuthService(t *testing.T) (*auth.Service, *repository.UserRepository, *repository.EmailVerificationRepository, *gorm.DB) {
	return newTestAuthServiceWithCaptcha(t, nil)
}

func newTestAuthServiceWithCaptcha(t *testing.T, cm auth.CaptchaManager) (*auth.Service, *repository.UserRepository, *repository.EmailVerificationRepository, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := db.AutoMigrate(&domain.User{}, &domain.EmailVerificationToken{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := repository.NewUserRepository(db)
	verificationRepo := repository.NewEmailVerificationRepository(db)
	tokenManager := token.NewJWTManager("test-secret", time.Minute, 24*time.Hour)
	refreshStore := token.NewMemoryRefreshTokenStore()
	service := auth.NewService(repo, verificationRepo, tokenManager, refreshStore, cm, testEmailSender{})

	return service, repo, verificationRepo, db
}

// TestAuthServiceRegisterAndLogin 覆盖注册成功、登录成功以及密码哈希与登录时间更新。
func TestAuthServiceRegisterAndLogin(t *testing.T) {
	svc, repo, _, _ := newTestAuthService(t)
	ctx := context.Background()

	user, tokens, err := svc.Register(ctx, auth.RegisterParams{
		Username:  "alice",
		Email:     "alice@example.com",
		Password:  "password123",
		AvatarURL: "https://cdn.example.com/avatar/alice.png",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := repo.MarkEmailVerified(ctx, user.ID, time.Now()); err != nil {
		t.Fatalf("mark email verified: %v", err)
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
		Identifier: "alice@example.com",
		Password:   "password123",
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
	if stored.AvatarURL != "https://cdn.example.com/avatar/alice.png" {
		t.Fatalf("expected avatar url to persist, got %s", stored.AvatarURL)
	}
}

// TestAuthServiceRegisterDuplicateEmailAndUsername 校验重复邮箱/用户名时返回对应错误。
func TestAuthServiceRegisterDuplicateEmailAndUsername(t *testing.T) {
	svc, _, _, _ := newTestAuthService(t)
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

	_, _, err = svc.Register(ctx, auth.RegisterParams{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
	})
	if !errors.Is(err, auth.ErrEmailAndUsernameTaken) {
		t.Fatalf("expected both email and username taken error, got %v", err)
	}
}

func TestAuthServiceRegisterRequiresCaptcha(t *testing.T) {
	fake := &fakeCaptcha{expectedID: "captcha-id", expectedCode: "13579"}
	svc, _, _, _ := newTestAuthServiceWithCaptcha(t, fake)
	ctx := context.Background()

	_, _, err := svc.Register(ctx, auth.RegisterParams{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
	})
	if !errors.Is(err, auth.ErrCaptchaRequired) {
		t.Fatalf("expected ErrCaptchaRequired, got %v", err)
	}

	fake.returnErr = captcha.ErrCaptchaMismatch

	_, _, err = svc.Register(ctx, auth.RegisterParams{
		Username:    "alice",
		Email:       "alice@example.com",
		Password:    "password123",
		CaptchaID:   "captcha-id",
		CaptchaCode: "wrong",
	})
	if !errors.Is(err, auth.ErrCaptchaInvalid) {
		t.Fatalf("expected ErrCaptchaInvalid, got %v", err)
	}

	fake.returnErr = nil
	fake.verifyCalls = 0

	_, _, err = svc.Register(ctx, auth.RegisterParams{
		Username:    "alice",
		Email:       "alice@example.com",
		Password:    "password123",
		CaptchaID:   "captcha-id",
		CaptchaCode: "13579",
	})
	if err != nil {
		t.Fatalf("register with captcha failed: %v", err)
	}
	if fake.verifyCalls == 0 {
		t.Fatalf("expected captcha verification to be invoked")
	}

}

// TestAuthServiceLoginInvalidCredentials 确认登录失败场景统一返回 ErrInvalidLogin。
func TestAuthServiceLoginInvalidCredentials(t *testing.T) {
	svc, repo, _, _ := newTestAuthService(t)
	ctx := context.Background()

	user, _, err := svc.Register(ctx, auth.RegisterParams{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := repo.MarkEmailVerified(ctx, user.ID, time.Now()); err != nil {
		t.Fatalf("mark email verified: %v", err)
	}

	_, _, err = svc.Login(ctx, auth.LoginParams{
		Identifier: "alice@example.com",
		Password:   "wrong-password",
	})
	if !errors.Is(err, auth.ErrInvalidLogin) {
		t.Fatalf("expected invalid login error, got %v", err)
	}

	_, _, err = svc.Login(ctx, auth.LoginParams{
		Identifier: "unknown@example.com",
		Password:   "password123",
	})
	if !errors.Is(err, auth.ErrInvalidLogin) {
		t.Fatalf("expected invalid login for unknown user, got %v", err)
	}
}

func TestAuthServiceLoginWithUsername(t *testing.T) {
	svc, repo, _, _ := newTestAuthService(t)
	ctx := context.Background()

	user, _, err := svc.Register(ctx, auth.RegisterParams{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := repo.MarkEmailVerified(ctx, user.ID, time.Now()); err != nil {
		t.Fatalf("mark email verified: %v", err)
	}

	loginUser, _, err := svc.Login(ctx, auth.LoginParams{
		Identifier: "alice",
		Password:   "password123",
	})
	if err != nil {
		t.Fatalf("login with username: %v", err)
	}
	if loginUser.ID != user.ID {
		t.Fatalf("expected login to return same user id")
	}
}

// TestAuthServiceRefreshAndLogout 覆盖刷新令牌与登出逻辑。
func TestAuthServiceRefreshAndLogout(t *testing.T) {
	svc, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	_, tokens, err := svc.Register(ctx, auth.RegisterParams{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if tokens.RefreshToken == "" {
		t.Fatalf("expected refresh token")
	}

	newTokens, err := svc.Refresh(ctx, tokens.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if newTokens.RefreshToken == tokens.RefreshToken {
		t.Fatalf("expected refresh token rotation")
	}

	if _, err := svc.Refresh(ctx, tokens.RefreshToken); !errors.Is(err, auth.ErrRefreshTokenRevoked) {
		t.Fatalf("expected revoked error, got %v", err)
	}

	if err := svc.Logout(ctx, newTokens.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}

	if _, err := svc.Refresh(ctx, newTokens.RefreshToken); !errors.Is(err, auth.ErrRefreshTokenRevoked) {
		t.Fatalf("expected revoked after logout, got %v", err)
	}

	if err := svc.Logout(ctx, "invalid"); !errors.Is(err, auth.ErrRefreshTokenInvalid) {
		t.Fatalf("expected invalid token error, got %v", err)
	}
}
