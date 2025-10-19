package unit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"electron-go-app/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

const (
	offlineMiddlewareTestUserID = uint(42)
)

// TestOfflineAuthMiddlewareHandle 确认离线鉴权中间件能够注入默认用户信息。
func TestOfflineAuthMiddlewareHandle(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(middleware.NewOfflineAuthMiddleware(offlineMiddlewareTestUserID, true).Handle())

	var (
		capturedUser interface{}
		capturedRole interface{}
	)

	router.GET("/", func(c *gin.Context) {
		capturedUser, _ = c.Get("userID")
		capturedRole, _ = c.Get("isAdmin")
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if userID, ok := capturedUser.(uint); !ok || userID != offlineMiddlewareTestUserID {
		t.Fatalf("userID not injected, got=%v", capturedUser)
	}
	if isAdmin, ok := capturedRole.(bool); !ok || !isAdmin {
		t.Fatalf("isAdmin not injected, got=%v", capturedRole)
	}
}

