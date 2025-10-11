package unit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	volc "electron-go-app/backend/internal/infra/model/volcengine"
)

func TestVolcengineClientChatCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request payload: %v", err)
		}
		if payload["model"] != "doubao" {
			t.Fatalf("expected model doubao, got %v", payload["model"])
		}
		resp := map[string]any{
			"id":      "volc-test",
			"object":  "chat.completion",
			"created": 100,
			"model":   "doubao",
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "hello",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":             5,
				"completion_tokens":         7,
				"total_tokens":              12,
				"prompt_tokens_details":     map[string]any{"cached_tokens": 0},
				"completion_tokens_details": map[string]any{"reasoning_tokens": 0},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := volc.NewClient("ark-test", volc.WithBaseURL(server.URL))
	response, err := client.ChatCompletion(context.Background(), volc.ChatCompletionRequest{
		Model: "doubao",
		Messages: []volc.ChatMessage{
			{Role: "system", Content: "You are assistant"},
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("chat completion failed: %v", err)
	}
	if len(response.Choices) == 0 || response.Choices[0].Message.Content != "hello" {
		t.Fatalf("unexpected response: %+v", response.Choices)
	}
}
