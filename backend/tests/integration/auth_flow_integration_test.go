//go:build integration

/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 17:11:16
 * @FilePath: \electron-go-app\backend\tests\integration\auth_flow_integration_test.go
 * @LastEditTime: 2025-10-09 17:11:22
 */

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/handler"
	response "electron-go-app/backend/internal/infra/common"
	"electron-go-app/backend/internal/infra/token"
	"electron-go-app/backend/internal/middleware"
	"electron-go-app/backend/internal/repository"
	"electron-go-app/backend/internal/server"
	authsvc "electron-go-app/backend/internal/service/auth"
	usersvc "electron-go-app/backend/internal/service/user"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type noopEmailSender struct{}

func (noopEmailSender) SendVerification(_ context.Context, _ *domain.User, _ string) error {
	return nil
}

func setupAuthFlow(t *testing.T) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.ReleaseMode)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&domain.User{}, &domain.EmailVerificationToken{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := repository.NewUserRepository(db)
	verificationRepo := repository.NewEmailVerificationRepository(db)
	secret := "integration-secret"
	jwtManager := token.NewJWTManager(secret, time.Minute*5, time.Hour)
	refreshStore := token.NewMemoryRefreshTokenStore()
	authService := authsvc.NewService(repo, verificationRepo, jwtManager, refreshStore, nil, noopEmailSender{})
	userService := usersvc.NewService(repo)

	authHandler := handler.NewAuthHandler(authService, nil, 0, 0)
	userHandler := handler.NewUserHandler(userService)
	authMW := middleware.NewAuthMiddleware(secret)

	return server.NewRouter(server.RouterOptions{
		AuthHandler: authHandler,
		UserHandler: userHandler,
		AuthMW:      authMW,
	})
}

func performJSONRequest(t *testing.T, router http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func TestAuthFlow_RegisterLoginAndFetchProfile(t *testing.T) {
	router := setupAuthFlow(t)

	email := "integration+" + time.Now().Format("150405.000") + "@example.com"
	password := "Passw0rd!"
	username := "user" + time.Now().Format("150405")

	// Step 1: register new user
	registerResp := performJSONRequest(t, router, http.MethodPost, "/api/auth/register", map[string]any{
		"username": username,
		"email":    email,
		"password": password,
	}, nil)

	if registerResp.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", registerResp.Code, registerResp.Body.String())
	}

	var registerBody response.Response
	if err := json.Unmarshal(registerResp.Body.Bytes(), &registerBody); err != nil {
		t.Fatalf("decode register body: %v", err)
	}

	if !registerBody.Success {
		t.Fatalf("register expected success=true: %s", registerResp.Body.String())
	}

	dataMap, ok := registerBody.Data.(map[string]any)
	if !ok {
		t.Fatalf("register data unexpected type: %T", registerBody.Data)
	}

	userMap, ok := dataMap["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user in register response")
	}
	if userMap["id"] == nil {
		t.Fatalf("register response missing user id")
	}

	tokensMap, ok := dataMap["tokens"].(map[string]any)
	if !ok {
		t.Fatalf("expected tokens in register response")
	}

	accessToken, ok := tokensMap["access_token"].(string)
	if !ok || accessToken == "" {
		t.Fatalf("expected access token in register response")
	}

	if tokensMap["refresh_token"] == "" {
		t.Fatalf("expected refresh token")
	}

	// Step 2: login should be blocked until email verified
	loginResp := performJSONRequest(t, router, http.MethodPost, "/api/auth/login", map[string]any{
		"email":    email,
		"password": password,
	}, nil)

	if loginResp.Code != http.StatusForbidden {
		t.Fatalf("expected login forbidden before verification, got %d body=%s", loginResp.Code, loginResp.Body.String())
	}

	// Step 3: request verification token
	requestResp := performJSONRequest(t, router, http.MethodPost, "/api/auth/request-email-verification", map[string]any{
		"email": email,
	}, nil)

	if requestResp.Code != http.StatusOK {
		t.Fatalf("request verification status = %d, body = %s", requestResp.Code, requestResp.Body.String())
	}

	var requestBody response.Response
	if err := json.Unmarshal(requestResp.Body.Bytes(), &requestBody); err != nil {
		t.Fatalf("decode request body: %v", err)
	}

	if !requestBody.Success {
		t.Fatalf("request verification expected success=true: %s", requestResp.Body.String())
	}

	reqData, ok := requestBody.Data.(map[string]any)
	if !ok {
		t.Fatalf("request verification data unexpected type: %T", requestBody.Data)
	}

	tokenVal, _ := reqData["token"].(string)
	if tokenVal == "" {
		t.Fatalf("expected token in request verification response")
	}

	// Step 4: verify email
	verifyResp := performJSONRequest(t, router, http.MethodPost, "/api/auth/verify-email", map[string]any{
		"token": tokenVal,
	}, nil)

	if verifyResp.Code != http.StatusNoContent {
		t.Fatalf("verify email status = %d, body = %s", verifyResp.Code, verifyResp.Body.String())
	}

	// Step 5: login after verification
	loginResp = performJSONRequest(t, router, http.MethodPost, "/api/auth/login", map[string]any{
		"email":    email,
		"password": password,
	}, nil)

	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginResp.Code, loginResp.Body.String())
	}

	var loginBody response.Response
	if err := json.Unmarshal(loginResp.Body.Bytes(), &loginBody); err != nil {
		t.Fatalf("decode login body: %v", err)
	}

	if !loginBody.Success {
		t.Fatalf("login expected success=true: %s", loginResp.Body.String())
	}

	loginData, ok := loginBody.Data.(map[string]any)
	if !ok {
		t.Fatalf("login data unexpected type: %T", loginBody.Data)
	}

	loginTokens, ok := loginData["tokens"].(map[string]any)
	if !ok {
		t.Fatalf("expected tokens in login response")
	}

	accessToken, _ = loginTokens["access_token"].(string)
	if accessToken == "" {
		t.Fatalf("login access token missing")
	}

	refreshToken, _ := loginTokens["refresh_token"].(string)
	if refreshToken == "" {
		t.Fatalf("login refresh token missing")
	}

	// Step 3: refresh token
	refreshResp := performJSONRequest(t, router, http.MethodPost, "/api/auth/refresh", map[string]any{
		"refresh_token": refreshToken,
	}, nil)

	if refreshResp.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, body = %s", refreshResp.Code, refreshResp.Body.String())
	}

	var refreshBody response.Response
	if err := json.Unmarshal(refreshResp.Body.Bytes(), &refreshBody); err != nil {
		t.Fatalf("decode refresh body: %v", err)
	}

	if !refreshBody.Success {
		t.Fatalf("refresh expected success=true: %s", refreshResp.Body.String())
	}

	refreshData, ok := refreshBody.Data.(map[string]any)
	if !ok {
		t.Fatalf("refresh data unexpected type: %T", refreshBody.Data)
	}

	refreshedTokens, ok := refreshData["tokens"].(map[string]any)
	if !ok {
		t.Fatalf("expected tokens in refresh response")
	}

	newAccessToken, _ := refreshedTokens["access_token"].(string)
	if newAccessToken == "" {
		t.Fatalf("refresh access token missing")
	}

	newRefreshToken, _ := refreshedTokens["refresh_token"].(string)
	if newRefreshToken == "" {
		t.Fatalf("refresh token missing in response")
	}

	if newRefreshToken == refreshToken {
		t.Fatalf("expected rotated refresh token")
	}

	// Step 4: call /api/users/me with JWT
	meResp := performJSONRequest(t, router, http.MethodGet, "/api/users/me", nil, map[string]string{
		"Authorization": "Bearer " + newAccessToken,
	})

	if meResp.Code != http.StatusOK {
		t.Fatalf("/me status = %d, body = %s", meResp.Code, meResp.Body.String())
	}

	var meBody response.Response
	if err := json.Unmarshal(meResp.Body.Bytes(), &meBody); err != nil {
		t.Fatalf("decode me body: %v", err)
	}

	if !meBody.Success {
		t.Fatalf("me expected success=true: %s", meResp.Body.String())
	}

	meData, ok := meBody.Data.(map[string]any)
	if !ok {
		t.Fatalf("me data unexpected type: %T", meBody.Data)
	}

	if meData["user"] == nil {
		t.Fatalf("me response missing user data")
	}

	// Step 5: logout
	logoutResp := performJSONRequest(t, router, http.MethodPost, "/api/auth/logout", map[string]any{
		"refresh_token": newRefreshToken,
	}, nil)

	if logoutResp.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, body = %s", logoutResp.Code, logoutResp.Body.String())
	}

	// Step 6: refresh with revoked token should fail
	revokedResp := performJSONRequest(t, router, http.MethodPost, "/api/auth/refresh", map[string]any{
		"refresh_token": newRefreshToken,
	}, nil)

	if revokedResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked refresh to be 401, got %d", revokedResp.Code)
	}
}
