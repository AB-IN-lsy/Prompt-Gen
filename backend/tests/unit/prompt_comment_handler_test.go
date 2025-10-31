package unit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	promptdomain "electron-go-app/backend/internal/domain/prompt"
	userdomain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/handler"
	"electron-go-app/backend/internal/repository"
	promptcommentsvc "electron-go-app/backend/internal/service/promptcomment"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPromptCommentHandlerDeleteRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:handler-delete-admin?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &promptdomain.Prompt{}, &promptdomain.PromptComment{}, &promptdomain.PromptCommentLike{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	commentRepo := repository.NewPromptCommentRepository(db)
	commentLikeRepo := repository.NewPromptCommentLikeRepository(db)
	promptRepo := repository.NewPromptRepository(db)
	userRepo := repository.NewUserRepository(db)
	commentService := promptcommentsvc.NewService(commentRepo, commentLikeRepo, promptRepo, userRepo, nil, nil, promptcommentsvc.Config{DefaultPageSize: 5, MaxPageSize: 10, MaxBodyLength: 500})
	commentHandler := handler.NewPromptCommentHandler(commentService, nil, handler.PromptCommentRateLimit{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/prompts/comments/1", nil)
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "1"})
	commentHandler.Delete(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", w.Code)
	}
}

func TestPromptCommentHandlerDeleteSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:handler-delete-success?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &promptdomain.Prompt{}, &promptdomain.PromptComment{}, &promptdomain.PromptCommentLike{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	commentRepo := repository.NewPromptCommentRepository(db)
	commentLikeRepo := repository.NewPromptCommentLikeRepository(db)
	promptRepo := repository.NewPromptRepository(db)
	userRepo := repository.NewUserRepository(db)
	commentService := promptcommentsvc.NewService(commentRepo, commentLikeRepo, promptRepo, userRepo, nil, nil, promptcommentsvc.Config{DefaultPageSize: 5, MaxPageSize: 10, MaxBodyLength: 500})
	commentHandler := handler.NewPromptCommentHandler(commentService, nil, handler.PromptCommentRateLimit{})

	ctx := context.Background()
	user := userdomain.User{Username: "admin", Email: "admin@example.com"}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	prompt := promptdomain.Prompt{
		UserID:           user.ID,
		Topic:            "题目",
		Body:             "正文",
		Instructions:     "说明",
		PositiveKeywords: "[]",
		NegativeKeywords: "[]",
		Model:            "deepseek",
		Status:           promptdomain.PromptStatusPublished,
		Tags:             "[]",
	}
	if err := db.WithContext(ctx).Create(&prompt).Error; err != nil {
		t.Fatalf("create prompt: %v", err)
	}
	comment, err := commentService.Create(ctx, promptcommentsvc.CreateCommentInput{PromptID: prompt.ID, UserID: user.ID, Body: "内容"})
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/prompts/comments/%d", comment.ID), nil)
	c.Params = append(c.Params, gin.Param{Key: "id", Value: fmt.Sprintf("%d", comment.ID)})
	c.Set("isAdmin", true)
	commentHandler.Delete(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := resp["data"].(map[string]any)
	if !ok || data["id"] == nil {
		t.Fatalf("unexpected response payload: %v", resp)
	}
	var count int64
	if err := db.WithContext(ctx).Model(&promptdomain.PromptComment{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected comments deleted, remaining=%d", count)
	}
}
