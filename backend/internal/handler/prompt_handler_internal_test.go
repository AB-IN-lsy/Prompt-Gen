package handler

import (
	"fmt"
	"testing"

	promptsvc "electron-go-app/backend/internal/service/prompt"
)

func TestExtractContentRejectReason_WithReason(t *testing.T) {
	err := fmt.Errorf("%w: %s", promptsvc.ErrContentRejected, "含有敏感词")
	got := extractContentRejectReason(err)
	if got != "含有敏感词" {
		t.Fatalf("expected reason to be '含有敏感词', got %q", got)
	}
}

func TestExtractContentRejectReason_WithoutColon(t *testing.T) {
	err := fmt.Errorf("%w", promptsvc.ErrContentRejected)
	got := extractContentRejectReason(err)
	expected := promptsvc.ErrContentRejected.Error()
	if got != expected {
		t.Fatalf("expected original message %q, got %q", expected, got)
	}
}

func TestExtractContentRejectReason_Empty(t *testing.T) {
	if got := extractContentRejectReason(nil); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}
