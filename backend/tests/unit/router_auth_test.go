/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 23:27:38
 * @FilePath: \electron-go-app\backend\tests\unit\router_auth_test.go
 * @LastEditTime: 2025-10-08 23:27:44
 */
package unit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	domain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/handler"
	"electron-go-app/backend/internal/infra/token"
	"electron-go-app/backend/internal/middleware"
	"electron-go-app/backend/internal/repository"
	"electron-go-app/backend/internal/server"
	authsvc "electron-go-app/backend/internal/service/auth"
	usersvc "electron-go-app/backend/internal/service/user"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type noopEmailSender struct{}

func (noopEmailSender) SendVerification(_ context.Context, _ *domain.User, _ string) error {
	return nil
}

type userProfileResponse struct {
	Success bool `json:"success"`
	Data    struct {
		User struct {
			ID uint `json:"id"`
		} `json:"user"`
	} `json:"data"`
}

// setupRouter 构建完整的 Router（含 Auth/User handler 与 JWT 中间件），用于 HTTP 单测。
func setupRouter(t *testing.T) (*gin.Engine, uint, string) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&domain.User{}, &domain.EmailVerificationToken{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := repository.NewUserRepository(db)
	modelRepo := repository.NewModelCredentialRepository(db)
	verificationRepo := repository.NewEmailVerificationRepository(db)

	user := &domain.User{
		Username: "router-user",
		Email:    "router@example.com",
	}
	if err := repo.Create(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	secret := "test-secret"
	jwtManager := token.NewJWTManager(secret, time.Minute, time.Hour)
	refreshStore := token.NewMemoryRefreshTokenStore()
	authService := authsvc.NewService(repo, verificationRepo, jwtManager, refreshStore, nil, noopEmailSender{})
	authHandler := handler.NewAuthHandler(authService, nil, 0, 0)

	userService := usersvc.NewService(repo, modelRepo)
	userHandler := handler.NewUserHandler(userService)

	authMW := middleware.NewAuthMiddleware(secret)

	router := server.NewRouter(server.RouterOptions{
		AuthHandler: authHandler,
		UserHandler: userHandler,
		AuthMW:      authMW,
	})

	return router, user.ID, secret
}

// buildRequest 简化测试请求构造，必要时附带 Authorization 头。
func buildRequest(method, path, token string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// signToken 使用给定密钥生成短期有效的 JWT，用于模拟合法用户请求。
func signToken(t *testing.T, secret string, userID uint) string {
	t.Helper()

	claims := jwt.MapClaims{
		"sub": strconv.FormatUint(uint64(userID), 10),
		"exp": time.Now().Add(5 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// TestRouter_UserEndpointsRequireJWT 覆盖未携带 Token 拦截、携带合法 Token 成功返回。
func TestRouter_UserEndpointsRequireJWT(t *testing.T) {
	router, userID, secret := setupRouter(t)

	// 未带 Token，应直接被中间件拦截为 401。
	t.Run("missing token returns 401", func(t *testing.T) {
		req := buildRequest(http.MethodGet, "/api/users/me", "")
		resp := httptest.NewRecorder()

		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", resp.Code)
		}
	})

	// 携带合法 Token，应顺利通过中间件并返回用户资料。
	t.Run("valid token returns profile", func(t *testing.T) {
		token := signToken(t, secret, userID)
		req := buildRequest(http.MethodGet, "/api/users/me", token)
		resp := httptest.NewRecorder()

		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.Code)
		}

		var body userProfileResponse
		if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		if !body.Success {
			t.Fatalf("expected success=true")
		}

		if body.Data.User.ID != userID {
			t.Fatalf("expected user id %d, got %d", userID, body.Data.User.ID)
		}
	})
}
