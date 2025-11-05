package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	promptdomain "electron-go-app/backend/internal/domain/prompt"
	response "electron-go-app/backend/internal/infra/common"
	deepseek "electron-go-app/backend/internal/infra/model/deepseek"
	"electron-go-app/backend/internal/repository"
	promptsvc "electron-go-app/backend/internal/service/prompt"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type handlerModelStub struct {
	responses []deepseek.ChatCompletionResponse
	requests  []deepseek.ChatCompletionRequest
	err       error
}

func (f *handlerModelStub) InvokeChatCompletion(ctx context.Context, userID uint, modelKey string, req deepseek.ChatCompletionRequest) (deepseek.ChatCompletionResponse, error) {
	f.requests = append(f.requests, req)
	if len(f.responses) == 0 {
		return deepseek.ChatCompletionResponse{}, f.err
	}
	res := f.responses[0]
	f.responses = f.responses[1:]
	return res, nil
}

func newTestService(t *testing.T, stub *handlerModelStub) (*promptsvc.Service, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&promptdomain.Prompt{}, &promptdomain.Keyword{}, &promptdomain.PromptKeyword{}, &promptdomain.PromptVersion{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	promptRepo := repository.NewPromptRepository(db)
	keywordRepo := repository.NewKeywordRepository(db)

	cfg := promptsvc.Config{
		KeywordLimit:        promptsvc.DefaultKeywordLimit,
		KeywordMaxLength:    promptsvc.DefaultKeywordMaxLength,
		TagLimit:            promptsvc.DefaultTagLimit,
		TagMaxLength:        promptsvc.DefaultTagMaxLength,
		DefaultListPageSize: promptsvc.DefaultPromptListPageSize,
		MaxListPageSize:     promptsvc.DefaultPromptListMaxPageSize,
		VersionRetention:    promptsvc.DefaultVersionRetentionLimit,
	}

	service, err := promptsvc.NewServiceWithConfig(promptRepo, keywordRepo, stub, nil, nil, nil, nil, nil, cfg)
	if err != nil {
		t.Fatalf("init prompt service: %v", err)
	}
	return service, db
}

func buildAuditResponsePayload(t *testing.T, allowed bool, reason string) deepseek.ChatCompletionResponse {
	t.Helper()
	payload := map[string]any{
		"allowed": allowed,
		"reason":  reason,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal audit payload: %v", err)
	}
	return deepseek.ChatCompletionResponse{
		Model: "audit-model",
		Choices: []deepseek.ChatCompletionChoice{
			{Message: deepseek.ChatMessage{Role: "assistant", Content: string(raw)}},
		},
	}
}

func buildInterpretationResponse(t *testing.T) deepseek.ChatCompletionResponse {
	t.Helper()
	payload := map[string]any{
		"topic":             "良性话题",
		"positive_keywords": []map[string]any{{"word": "React", "weight": 5}},
		"negative_keywords": []map[string]any{},
		"confidence":        0.9,
		"instructions":      "保持专业",
		"tags":              []string{"面试"},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal interpretation payload: %v", err)
	}
	return deepseek.ChatCompletionResponse{
		Model: "deepseek-chat",
		Choices: []deepseek.ChatCompletionChoice{
			{Message: deepseek.ChatMessage{Role: "assistant", Content: string(raw)}},
		},
	}
}

func TestInterpret_AuditPass(t *testing.T) {
	stub := &handlerModelStub{
		responses: []deepseek.ChatCompletionResponse{
			buildAuditResponsePayload(t, true, ""),
			buildInterpretationResponse(t),
		},
	}
	service, db := newTestService(t, stub)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})

	handler := NewPromptHandler(service, nil, PromptRateLimit{})
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", uint(1))
		c.Next()
	})
	router.POST("/interpret", handler.Interpret)

	body := `{"description":"合法描述","model_key":"deepseek-chat"}`
	req := httptest.NewRequest(http.MethodPost, "/interpret", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Topic string `json:"topic"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Success || resp.Data.Topic != "良性话题" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestInterpret_AuditRejected(t *testing.T) {
	stub := &handlerModelStub{
		responses: []deepseek.ChatCompletionResponse{
			buildAuditResponsePayload(t, false, "文本命中敏感词"),
		},
	}
	service, db := newTestService(t, stub)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})

	handler := NewPromptHandler(service, nil, PromptRateLimit{})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", uint(1))
		c.Next()
	})
	router.POST("/interpret", handler.Interpret)

	body := `{"description":"敏感文本","model_key":"deepseek-chat"}`
	req := httptest.NewRequest(http.MethodPost, "/interpret", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var resp struct {
		Success bool `json:"success"`
		Error   struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Code != string(response.ErrContentRejected) {
		t.Fatalf("expected CONTENT_REJECTED, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "文本命中敏感词" {
		t.Fatalf("unexpected message: %s", resp.Error.Message)
	}
	if reason, ok := resp.Error.Details["reason"].(string); !ok || reason != "文本命中敏感词" {
		t.Fatalf("expected reason detail, got %#v", resp.Error.Details)
	}
}

func TestGenerate_AuditSuccess(t *testing.T) {
	stub := &handlerModelStub{
		responses: []deepseek.ChatCompletionResponse{
			{
				Model: "deepseek-chat",
				Choices: []deepseek.ChatCompletionChoice{
					{Message: deepseek.ChatMessage{Role: "assistant", Content: "这是合规 Prompt"}},
				},
			},
			buildAuditResponsePayload(t, true, ""),
		},
	}
	service, db := newTestService(t, stub)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})

	handler := NewPromptHandler(service, nil, PromptRateLimit{})
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", uint(1))
		c.Next()
	})
	router.POST("/generate", handler.GeneratePrompt)

	body := `{"topic":"合法主题","model_key":"deepseek-chat","positive_keywords":[{"word":"React","polarity":"positive","weight":5}],"negative_keywords":[],"instructions":"保持专业"}`
	req := httptest.NewRequest(http.MethodPost, "/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Prompt string `json:"prompt"`
			Model  string `json:"model"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Success || resp.Data.Prompt == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestGenerate_AuditRejected(t *testing.T) {
	stub := &handlerModelStub{
		responses: []deepseek.ChatCompletionResponse{
			{
				Model: "deepseek-chat",
				Choices: []deepseek.ChatCompletionChoice{
					{Message: deepseek.ChatMessage{Role: "assistant", Content: "违规 Prompt"}},
				},
			},
			buildAuditResponsePayload(t, false, "涉及违规内容"),
		},
	}
	service, db := newTestService(t, stub)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})

	handler := NewPromptHandler(service, nil, PromptRateLimit{})
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", uint(1))
		c.Next()
	})
	router.POST("/generate", handler.GeneratePrompt)

	body := `{"topic":"违规主题","model_key":"deepseek-chat","positive_keywords":[{"word":"React","polarity":"positive","weight":5}],"negative_keywords":[],"instructions":"保持专业"}`
	req := httptest.NewRequest(http.MethodPost, "/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var resp struct {
		Success bool `json:"success"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error.Code != string(response.ErrContentRejected) {
		t.Fatalf("expected CONTENT_REJECTED, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "涉及违规内容" {
		t.Fatalf("unexpected message: %s", resp.Error.Message)
	}
}
