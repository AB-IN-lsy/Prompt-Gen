package unit

import (
	response "electron-go-app/backend/internal/infra/common"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResponseSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	data := gin.H{"hello": "world"}
	meta := response.MetaPagination{Page: 1, PageSize: 10, TotalItems: 20, TotalPages: 2}

	response.Success(ctx, http.StatusAccepted, data, meta)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, recorder.Code)
	}

	var body response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}

	if !body.Success {
		t.Fatalf("expected success=true")
	}

	payload, ok := body.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data map, got %T", body.Data)
	}
	if payload["hello"] != "world" {
		t.Fatalf("unexpected data: %v", payload)
	}

	metaMap, ok := body.Meta.(map[string]any)
	if !ok {
		t.Fatalf("expected meta map, got %T", body.Meta)
	}
	if metaMap["page"].(float64) != 1 {
		t.Fatalf("unexpected meta: %v", metaMap)
	}
}

func TestResponseFail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	response.Fail(ctx, http.StatusBadRequest, response.ErrBadRequest, "invalid payload", gin.H{"field": "email"})

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}

	var body response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}

	if body.Success {
		t.Fatalf("expected success=false")
	}

	if body.Error == nil {
		t.Fatalf("expected error struct")
	}
	if body.Error.Code != response.ErrBadRequest {
		t.Fatalf("unexpected code: %s", body.Error.Code)
	}
	if body.Error.Message != "invalid payload" {
		t.Fatalf("unexpected message: %s", body.Error.Message)
	}

	details, ok := body.Error.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected details map, got %T", body.Error.Details)
	}
	if details["field"] != "email" {
		t.Fatalf("unexpected details: %v", details)
	}
}
