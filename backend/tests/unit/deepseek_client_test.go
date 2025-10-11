package unit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"electron-go-app/backend/internal/infra/model/deepseek"
	modelsvc "electron-go-app/backend/internal/service/model"
)

func TestDeepSeekClientChatCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("expected authorization header, got %s", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request payload: %v", err)
		}
		if payload["model"] != "deepseek-chat" {
			t.Fatalf("expected model deepseek-chat, got %v", payload["model"])
		}
		if _, ok := payload["messages"]; !ok {
			t.Fatalf("expected messages field")
		}

		response := map[string]any{
			"id":      "test-id",
			"object":  "chat.completion",
			"created": 123456789,
			"model":   "deepseek-chat",
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Hello from DeepSeek!",
					},
					"finish_reason": "stop",
					"logprobs":      nil,
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 15,
				"total_tokens":      25,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := deepseek.NewClient("sk-test", deepseek.WithBaseURL(server.URL))
	response, err := client.ChatCompletion(context.Background(), deepseek.ChatCompletionRequest{
		Model: "deepseek-chat",
		Messages: []deepseek.ChatMessage{
			{Role: "system", Content: "You are an assistant."},
			{Role: "user", Content: "Ping?"},
		},
	})
	if err != nil {
		t.Fatalf("chat completion failed: %v", err)
	}
	if response.ID != "test-id" {
		t.Fatalf("unexpected response id: %s", response.ID)
	}
	if len(response.Choices) != 1 {
		t.Fatalf("expected single choice, got %d", len(response.Choices))
	}
	if response.Choices[0].Message.Content != "Hello from DeepSeek!" {
		t.Fatalf("unexpected content: %s", response.Choices[0].Message.Content)
	}
	if response.Usage == nil || response.Usage.TotalTokens != 25 {
		t.Fatalf("expected usage total tokens 25")
	}
}

func TestDeepSeekClientAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "invalid api key",
				"type":    "authentication_error",
				"code":    "invalid_key",
			},
		})
	}))
	defer server.Close()

	client := deepseek.NewClient("sk-test", deepseek.WithBaseURL(server.URL))
	_, err := client.ChatCompletion(context.Background(), deepseek.ChatCompletionRequest{
		Model: "deepseek-chat",
		Messages: []deepseek.ChatMessage{
			{Role: "user", Content: "test"},
		},
	})
	if err == nil {
		t.Fatalf("expected api error, got nil")
	}
	apiErr, ok := err.(*deepseek.APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", apiErr.StatusCode)
	}
	if apiErr.Code != "invalid_key" {
		t.Fatalf("expected code invalid_key, got %s", apiErr.Code)
	}
	if apiErr.Type != "authentication_error" {
		t.Fatalf("expected type authentication_error, got %s", apiErr.Type)
	}
}

func TestInvokeDeepSeekChatCompletion(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	os.Setenv("MODEL_CREDENTIAL_MASTER_KEY", base64.StdEncoding.EncodeToString(key))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request payload: %v", err)
		}
		if payload["model"] != "deepseek-chat" {
			t.Fatalf("expected model deepseek-chat, got %v", payload["model"])
		}
		if payload["max_tokens"] != float64(4096) {
			t.Fatalf("expected max_tokens 4096, got %v", payload["max_tokens"])
		}
		if payload["temperature"] != float64(1) {
			t.Fatalf("expected temperature 1, got %v", payload["temperature"])
		}

		response := map[string]any{
			"id":      "invoke-test",
			"object":  "chat.completion",
			"created": 111,
			"model":   "deepseek-chat",
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "OK",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 2,
				"total_tokens":      12,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	svc, _, _, userID := newTestModelService(t)

	_, err := svc.Create(context.Background(), userID, modelsvc.CreateInput{
		Provider:    "deepseek",
		ModelKey:    "deepseek-chat",
		DisplayName: "DeepSeek Chat",
		BaseURL:     server.URL,
		APIKey:      "sk-secret",
		ExtraConfig: map[string]any{
			"max_tokens":      4096,
			"temperature":     1,
			"response_format": map[string]any{"type": "text"},
		},
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	resp, err := svc.InvokeDeepSeekChatCompletion(context.Background(), userID, "deepseek-chat", deepseek.ChatCompletionRequest{
		Messages: []deepseek.ChatMessage{
			{Role: "user", Content: "Hi"},
		},
	})
	if err != nil {
		t.Fatalf("invoke deepseek: %v", err)
	}
	if resp.ID != "invoke-test" {
		t.Fatalf("unexpected response id: %s", resp.ID)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "OK" {
		t.Fatalf("unexpected response content: %+v", resp.Choices)
	}
}

func TestServiceTestConnection(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 2)
	}
	os.Setenv("MODEL_CREDENTIAL_MASTER_KEY", base64.StdEncoding.EncodeToString(key))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request payload: %v", err)
		}
		if payload["model"] != "deepseek-chat" {
			t.Fatalf("expected model deepseek-chat, got %v", payload["model"])
		}

		response := map[string]any{
			"id":      "test-connection",
			"object":  "chat.completion",
			"created": 999,
			"model":   "deepseek-chat",
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "pong",
					},
					"finish_reason": "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	svc, _, _, userID := newTestModelService(t)
	credential, err := svc.Create(context.Background(), userID, modelsvc.CreateInput{
		Provider:    "deepseek",
		ModelKey:    "deepseek-chat",
		DisplayName: "DeepSeek",
		BaseURL:     server.URL,
		APIKey:      "sk-test",
		ExtraConfig: map[string]any{},
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	resp, err := svc.TestConnection(context.Background(), userID, credential.ID, deepseek.ChatCompletionRequest{
		Messages: []deepseek.ChatMessage{{Role: "user", Content: "ping"}},
	})
	if err != nil {
		t.Fatalf("test connection: %v", err)
	}
	if resp.ID != "test-connection" {
		t.Fatalf("unexpected response id: %s", resp.ID)
	}

	list, err := svc.List(context.Background(), userID)
	if err != nil {
		t.Fatalf("list credentials: %v", err)
	}
	found := false
	for _, item := range list {
		if item.ID == credential.ID {
			found = true
			if item.LastVerifiedAt == nil {
				t.Fatalf("expected last_verified_at to be set")
			}
		}
	}
	if !found {
		t.Fatalf("credential not found in list result")
	}
}

func TestServiceTestConnectionDisabled(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 3)
	}
	os.Setenv("MODEL_CREDENTIAL_MASTER_KEY", base64.StdEncoding.EncodeToString(key))

	svc, _, _, userID := newTestModelService(t)
	credential, err := svc.Create(context.Background(), userID, modelsvc.CreateInput{
		Provider:    "deepseek",
		ModelKey:    "deepseek-chat",
		DisplayName: "DeepSeek",
		BaseURL:     "",
		APIKey:      "sk-disabled",
		ExtraConfig: map[string]any{},
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	status := "disabled"
	if _, err := svc.Update(context.Background(), userID, credential.ID, modelsvc.UpdateInput{Status: &status}); err != nil {
		t.Fatalf("disable credential: %v", err)
	}

	_, err = svc.TestConnection(context.Background(), userID, credential.ID, deepseek.ChatCompletionRequest{
		Messages: []deepseek.ChatMessage{{Role: "user", Content: "ping"}},
	})
	if !errors.Is(err, modelsvc.ErrCredentialDisabled) {
		t.Fatalf("expected ErrCredentialDisabled, got %v", err)
	}
}

func TestInvokeVolcengineChatCompletion(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 4)
	}
	os.Setenv("MODEL_CREDENTIAL_MASTER_KEY", base64.StdEncoding.EncodeToString(key))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request payload: %v", err)
		}
		if payload["model"] != "doubao-1-5-thinking-pro-250415" {
			t.Fatalf("unexpected model: %v", payload["model"])
		}
		response := map[string]any{
			"id":      "volc-test",
			"object":  "chat.completion",
			"created": 1234,
			"model":   "doubao-1-5-thinking-pro-250415",
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "pong",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":             20,
				"completion_tokens":         10,
				"total_tokens":              30,
				"prompt_tokens_details":     map[string]any{"cached_tokens": 0},
				"completion_tokens_details": map[string]any{"reasoning_tokens": 0},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	svc, _, _, userID := newTestModelService(t)
	cred, err := svc.Create(context.Background(), userID, modelsvc.CreateInput{
		Provider:    "volcengine",
		ModelKey:    "doubao-1-5-thinking-pro-250415",
		DisplayName: "Volcengine Doubao",
		BaseURL:     server.URL,
		APIKey:      "sk-volcano",
		ExtraConfig: map[string]any{},
	})
	if err != nil {
		t.Fatalf("create volcengine credential: %v", err)
	}

	resp, err := svc.InvokeDeepSeekChatCompletion(context.Background(), userID, cred.ModelKey, deepseek.ChatCompletionRequest{
		Messages: []deepseek.ChatMessage{{Role: "user", Content: "你好"}},
	})
	if err != nil {
		t.Fatalf("invoke volcengine: %v", err)
	}
	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content != "pong" {
		t.Fatalf("unexpected volcengine response: %+v", resp.Choices)
	}

	resp2, err := svc.TestConnection(context.Background(), userID, cred.ID, deepseek.ChatCompletionRequest{
		Messages: []deepseek.ChatMessage{{Role: "user", Content: "test"}},
	})
	if err != nil {
		t.Fatalf("test connection for volcengine: %v", err)
	}
	if resp2.ID == "" {
		t.Fatalf("expected response id set for volcengine test")
	}

	list, err := svc.List(context.Background(), userID)
	if err != nil {
		t.Fatalf("list credentials: %v", err)
	}
	found := false
	for _, item := range list {
		if item.ID == cred.ID {
			found = true
			if item.LastVerifiedAt == nil {
				t.Fatalf("expected volcengine credential last_verified_at set")
			}
		}
	}
	if !found {
		t.Fatalf("volcengine credential not found in list")
	}
}

func TestInvokeDeepSeekWithDisabledCredential(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	os.Setenv("MODEL_CREDENTIAL_MASTER_KEY", base64.StdEncoding.EncodeToString(key))

	svc, _, _, userID := newTestModelService(t)
	credential, err := svc.Create(context.Background(), userID, modelsvc.CreateInput{
		Provider:    "deepseek",
		ModelKey:    "deepseek-chat",
		DisplayName: "DeepSeek",
		BaseURL:     "",
		APIKey:      "sk-disabled",
		ExtraConfig: map[string]any{},
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	status := "disabled"
	if _, err := svc.Update(context.Background(), userID, credential.ID, modelsvc.UpdateInput{Status: &status}); err != nil {
		t.Fatalf("disable credential: %v", err)
	}

	_, err = svc.InvokeDeepSeekChatCompletion(context.Background(), userID, "deepseek-chat", deepseek.ChatCompletionRequest{
		Messages: []deepseek.ChatMessage{
			{Role: "user", Content: "test"},
		},
	})
	if !errors.Is(err, modelsvc.ErrCredentialDisabled) {
		t.Fatalf("expected ErrCredentialDisabled, got %v", err)
	}
}
