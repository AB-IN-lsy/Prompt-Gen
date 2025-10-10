//go:build e2e

// 真实环境链路测试：复用业务 Handler，直连远程 MySQL / Redis / Nacos。
// 注意：不像 main.go 那样启动整套服务，这里在测试进程内搭建 Gin router，
// 方便掌控生命周期，也能在失败时立刻释放连接、清理数据。

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	domain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/handler"
	"electron-go-app/backend/internal/infra/client"
	response "electron-go-app/backend/internal/infra/common"
	"electron-go-app/backend/internal/infra/ratelimit"
	"electron-go-app/backend/internal/infra/token"
	"electron-go-app/backend/internal/middleware"
	"electron-go-app/backend/internal/repository"
	"electron-go-app/backend/internal/server"
	authsvc "electron-go-app/backend/internal/service/auth"
	usersvc "electron-go-app/backend/internal/service/user"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// requireE2EEnabled 通过 env 开关避免在本地/CI 无意触发真实环境。
func requireE2EEnabled(t *testing.T) {
	if os.Getenv("E2E") != "1" {
		t.Skip("e2e tests disabled")
	}
}

// loadRemoteMySQLConfig 优先读取显式配置，其次走 Nacos 获取线上数据源。
func loadRemoteMySQLConfig(t *testing.T, ctx context.Context) client.MySQLConfig {
	t.Helper()

	if cfg, ok := mysqlConfigFromEnv(); ok {
		return cfg
	}

	opts, err := client.NewDefaultNacosOptions()
	if err != nil {
		t.Fatalf("build nacos options: %v", err)
	}

	group := os.Getenv("MYSQL_CONFIG_GROUP")
	if group == "" {
		group = "DEFAULT_GROUP"
	}

	cfg, err := client.LoadMySQLConfig(ctx, opts, group)
	if err != nil {
		t.Fatalf("load mysql config: %v", err)
	}

	return cfg
}

// mysqlConfigFromEnv 解析直连配置，便于在测试环境覆盖默认值。
func mysqlConfigFromEnv() (client.MySQLConfig, bool) {
	host := os.Getenv("MYSQL_TEST_HOST")
	user := os.Getenv("MYSQL_TEST_USER")
	pass := os.Getenv("MYSQL_TEST_PASS")
	dbName := os.Getenv("MYSQL_TEST_DB")

	if host == "" || user == "" || pass == "" || dbName == "" {
		return client.MySQLConfig{}, false
	}

	cfg := client.MySQLConfig{
		Host:     host,
		Username: user,
		Password: pass,
		Database: dbName,
		Params:   os.Getenv("MYSQL_TEST_PARAMS"),
	}

	return cfg, true
}

// connectRedis 直连远端 Redis 并执行一次写入作为连通性验证。
func connectRedis(t *testing.T, ctx context.Context) *redis.Client {
	t.Helper()

	opts, err := client.NewDefaultRedisOptions()
	if err != nil {
		t.Fatalf("build redis options: %v", err)
	}

	redisClient, err := client.NewRedisClient(opts)
	if err != nil {
		t.Fatalf("connect redis: %v", err)
	}

	key := fmt.Sprintf("e2e:redis:ping:%d", time.Now().UnixNano())
	if err := redisClient.Set(ctx, key, "ok", time.Minute).Err(); err != nil {
		t.Fatalf("redis set: %v", err)
	}
	t.Cleanup(func() { _ = redisClient.Del(context.Background(), key).Err() })
	t.Cleanup(func() { _ = redisClient.Close() })

	return redisClient
}

// setupRemoteRouter 复用生产 Handler，但不真正启动 HTTP 端口。
func setupRemoteRouter(t *testing.T, userRepo *repository.UserRepository, verificationRepo *repository.EmailVerificationRepository, redisClient *redis.Client, secret string) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.ReleaseMode)

	jwtManager := token.NewJWTManager(secret, 5*time.Minute, 24*time.Hour)
	refreshStore := token.NewRedisRefreshTokenStore(redisClient, "")
	limiter := ratelimit.NewRedisLimiter(redisClient, "verify_email")
	authService := authsvc.NewService(userRepo, verificationRepo, jwtManager, refreshStore, nil, nil)
	userService := usersvc.NewService(userRepo)

	authHandler := handler.NewAuthHandler(authService, limiter, 0, 0)
	userHandler := handler.NewUserHandler(userService)
	authMW := middleware.NewAuthMiddleware(secret)

	return server.NewRouter(server.RouterOptions{
		AuthHandler: authHandler,
		UserHandler: userHandler,
		AuthMW:      authMW,
	})
}

// performJSONRequest 保证测试中发起的 HTTP 调用一致，便于定位失败上下文。
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

// TestRemoteAuthFlow 覆盖真实环境的核心鉴权链路：
// 1. 直连 Redis、MySQL/Nacos，验证基础依赖可用；
// 2. 调用 /api/auth/register 在远程库中创建用户；
// 3. 使用 /api/auth/login 获取 JWT；
// 4. 调用 /api/users/me 执行设置更新；
// 5. 再次拉取 /api/users/me 校验最新资料。
func TestRemoteAuthFlow(t *testing.T) {
	requireE2EEnabled(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Step 0: 验证远端 Redis 是否可用（基础依赖探活）。
	redisClient := connectRedis(t, ctx)

	// Step 1: 通过 ENV / Nacos 解析 MySQL 配置，与线上数据库建立连接。
	cfg := loadRemoteMySQLConfig(t, ctx)
	t.Logf("connecting mysql host=%s db=%s", cfg.Host, cfg.Database)
	gormDB, sqlDB, err := client.NewGORMMySQL(cfg)
	if err != nil {
		t.Fatalf("connect mysql: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	repo := repository.NewUserRepository(gormDB)
	verificationRepo := repository.NewEmailVerificationRepository(gormDB)

	secret := os.Getenv("E2E_JWT_SECRET")
	if secret == "" {
		secret = fmt.Sprintf("e2e-secret-%d", time.Now().UnixNano())
	}

	router := setupRemoteRouter(t, repo, verificationRepo, redisClient, secret)

	randSuffix := rand.New(rand.NewSource(time.Now().UnixNano())).Int63()
	email := fmt.Sprintf("e2e+%d@example.com", randSuffix)
	username := fmt.Sprintf("e2euser%d", randSuffix)
	t.Logf("created user email=%s username=%s", email, username)
	password := "Passw0rd!"

	// 默认测试结束后清理远程库中的临时账号，避免污染生产数据。
	// 若需要人工检查数据库，可设置 E2E_KEEP_DATA=1 临时保留记录。
	if os.Getenv("E2E_KEEP_DATA") != "1" {
		t.Cleanup(func() {
			ctx := context.Background()
			if err := gormDB.WithContext(ctx).Where("email = ?", email).Delete(&domain.User{}).Error; err != nil {
				t.Logf("cleanup warning: %v", err)
			}
		})
	}

	// Step 2: 向真实环境发起注册，期望落库并返回 access/refresh token。
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
		t.Fatalf("decode register response: %v", err)
	}

	if !registerBody.Success {
		t.Fatalf("register failed: %s", registerResp.Body.String())
	}

	data, ok := registerBody.Data.(map[string]any)
	if !ok {
		t.Fatalf("register data unexpected type: %T", registerBody.Data)
	}

	tokens, ok := data["tokens"].(map[string]any)
	if !ok {
		t.Fatalf("register response missing tokens")
	}

	accessToken, _ := tokens["access_token"].(string)
	if accessToken == "" {
		t.Fatalf("register response missing access token")
	}

	// Step 3: 登录应在验证邮箱前被拒绝，随后完成验证再尝试。
	loginResp := performJSONRequest(t, router, http.MethodPost, "/api/auth/login", map[string]any{
		"email":    email,
		"password": password,
	}, nil)

	if loginResp.Code != http.StatusForbidden {
		t.Fatalf("expected login forbidden before verification, got %d body=%s", loginResp.Code, loginResp.Body.String())
	}

	requestResp := performJSONRequest(t, router, http.MethodPost, "/api/auth/request-email-verification", map[string]any{
		"email": email,
	}, nil)

	if requestResp.Code != http.StatusOK {
		t.Fatalf("request verification status = %d, body = %s", requestResp.Code, requestResp.Body.String())
	}

	var requestBody response.Response
	if err := json.Unmarshal(requestResp.Body.Bytes(), &requestBody); err != nil {
		t.Fatalf("decode request verification response: %v", err)
	}

	if !requestBody.Success {
		t.Fatalf("request verification failed: %s", requestResp.Body.String())
	}

	reqData, ok := requestBody.Data.(map[string]any)
	if !ok {
		t.Fatalf("request verification data unexpected type: %T", requestBody.Data)
	}

	tokenVal, _ := reqData["token"].(string)
	if tokenVal == "" {
		t.Fatalf("expected token in verification response")
	}

	verifyResp := performJSONRequest(t, router, http.MethodPost, "/api/auth/verify-email", map[string]any{
		"token": tokenVal,
	}, nil)

	if verifyResp.Code != http.StatusNoContent {
		t.Fatalf("verify email status = %d, body = %s", verifyResp.Code, verifyResp.Body.String())
	}

	loginResp = performJSONRequest(t, router, http.MethodPost, "/api/auth/login", map[string]any{
		"email":    email,
		"password": password,
	}, nil)

	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginResp.Code, loginResp.Body.String())
	}

	var loginBody response.Response
	if err := json.Unmarshal(loginResp.Body.Bytes(), &loginBody); err != nil {
		t.Fatalf("decode login response: %v", err)
	}

	if !loginBody.Success {
		t.Fatalf("login failed: %s", loginResp.Body.String())
	}

	loginData, ok := loginBody.Data.(map[string]any)
	if !ok {
		t.Fatalf("login data unexpected type: %T", loginBody.Data)
	}

	loginTokens, ok := loginData["tokens"].(map[string]any)
	if !ok {
		t.Fatalf("login response missing tokens")
	}

	accessToken, _ = loginTokens["access_token"].(string)
	if accessToken == "" {
		t.Fatalf("login response missing access token")
	}

	// Step 4: 调用 /api/users/me 执行设置更新，模拟前端修改偏好模型与同步开关。
	updateResp := performJSONRequest(t, router, http.MethodPut, "/api/users/me", map[string]any{
		"preferred_model": "claude-3-opus",
		"sync_enabled":    true,
	}, map[string]string{
		"Authorization": "Bearer " + accessToken,
	})

	if updateResp.Code != http.StatusOK {
		t.Fatalf("update me status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	var updateBody response.Response
	if err := json.Unmarshal(updateResp.Body.Bytes(), &updateBody); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if !updateBody.Success {
		t.Fatalf("update me failed: %s", updateResp.Body.String())
	}

	updatedData, ok := updateBody.Data.(map[string]any)
	if !ok {
		t.Fatalf("update data unexpected type: %T", updateBody.Data)
	}

	settingsMap, ok := updatedData["settings"].(map[string]any)
	if !ok {
		t.Fatalf("update response missing settings")
	}
	if model, _ := settingsMap["preferred_model"].(string); model != "claude-3-opus" {
		t.Fatalf("expected preferred_model to be claude-3-opus, got %v", settingsMap["preferred_model"])
	}
	if enabled, _ := settingsMap["sync_enabled"].(bool); !enabled {
		t.Fatalf("expected sync_enabled true, got %v", settingsMap["sync_enabled"])
	}

	// Step 5: 再次获取 /api/users/me，验证资料与设置已在远程库中生效。
	meResp := performJSONRequest(t, router, http.MethodGet, "/api/users/me", nil, map[string]string{
		"Authorization": "Bearer " + accessToken,
	})

	if meResp.Code != http.StatusOK {
		t.Fatalf("/me status = %d, body = %s", meResp.Code, meResp.Body.String())
	}

	var meBody response.Response
	if err := json.Unmarshal(meResp.Body.Bytes(), &meBody); err != nil {
		t.Fatalf("decode me response: %v", err)
	}

	if !meBody.Success {
		t.Fatalf("me failed: %s", meResp.Body.String())
	}

	meData, ok := meBody.Data.(map[string]any)
	if !ok {
		t.Fatalf("me data unexpected type: %T", meBody.Data)
	}

	userSection, ok := meData["user"].(map[string]any)
	if !ok {
		t.Fatalf("me response missing user data")
	}

	if _, ok := userSection["id"].(float64); !ok {
		t.Fatalf("me response missing user id: %v", userSection)
	}

	if meSettings, ok := meData["settings"].(map[string]any); ok {
		if model, _ := meSettings["preferred_model"].(string); model != "claude-3-opus" {
			t.Fatalf("expected preferred_model claude-3-opus after reload, got %v", model)
		}
		if enabled, _ := meSettings["sync_enabled"].(bool); !enabled {
			t.Fatalf("expected sync_enabled true after reload, got %v", enabled)
		}
	} else {
		t.Fatalf("me response missing settings block")
	}
}
