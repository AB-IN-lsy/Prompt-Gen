package unit

import (
	"context"
	"errors"
	"testing"

	promptdomain "electron-go-app/backend/internal/domain/prompt"
	userdomain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/repository"
	promptcommentsvc "electron-go-app/backend/internal/service/promptcomment"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupPromptCommentService(t *testing.T, cfg promptcommentsvc.Config, audit promptcommentsvc.AuditFunc) (*promptcommentsvc.Service, *gorm.DB) {
	t.Helper()
	dsn := "file:prompt-comment-service?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &promptdomain.Prompt{}, &promptdomain.PromptComment{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	commentRepo := repository.NewPromptCommentRepository(db)
	promptRepo := repository.NewPromptRepository(db)
	userRepo := repository.NewUserRepository(db)
	logger := zap.NewNop().Sugar()
	service := promptcommentsvc.NewService(commentRepo, promptRepo, userRepo, audit, logger, cfg)
	return service, db
}

func TestPromptCommentServiceCreateTopLevel(t *testing.T) {
	service, db := setupPromptCommentService(t, promptcommentsvc.Config{
		DefaultPageSize: 5,
		MaxPageSize:     20,
		RequireApproval: false,
		MaxBodyLength:   500,
	}, nil)
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	ctx := context.Background()
	user := userdomain.User{
		Username: "alice",
		Email:    "alice@example.com",
	}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	prompt := promptdomain.Prompt{
		UserID:           user.ID,
		Topic:            "测试话题",
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
	comment, err := service.Create(ctx, promptcommentsvc.CreateCommentInput{
		PromptID: prompt.ID,
		UserID:   user.ID,
		Body:     "第一条评论",
	})
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}
	if comment.RootID == nil || *comment.RootID != comment.ID {
		t.Fatalf("unexpected root id: %+v", comment.RootID)
	}
	if comment.Status != promptdomain.PromptCommentStatusApproved {
		t.Fatalf("unexpected status: %s", comment.Status)
	}
	if comment.Author == nil || comment.Author.Username != user.Username {
		t.Fatalf("author not attached: %+v", comment.Author)
	}
}

func TestPromptCommentServiceCreateReply(t *testing.T) {
	service, db := setupPromptCommentService(t, promptcommentsvc.Config{
		DefaultPageSize: 5,
		MaxPageSize:     20,
		RequireApproval: false,
		MaxBodyLength:   500,
	}, nil)
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	ctx := context.Background()
	user1 := userdomain.User{Username: "author", Email: "author@example.com"}
	user2 := userdomain.User{Username: "bob", Email: "bob@example.com"}
	if err := db.WithContext(ctx).Create(&user1).Error; err != nil {
		t.Fatalf("create user1: %v", err)
	}
	if err := db.WithContext(ctx).Create(&user2).Error; err != nil {
		t.Fatalf("create user2: %v", err)
	}
	prompt := promptdomain.Prompt{
		UserID:           user1.ID,
		Topic:            "话题",
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
	top, err := service.Create(ctx, promptcommentsvc.CreateCommentInput{
		PromptID: prompt.ID,
		UserID:   user2.ID,
		Body:     "顶层评论",
	})
	if err != nil {
		t.Fatalf("create top comment: %v", err)
	}
	reply, err := service.Create(ctx, promptcommentsvc.CreateCommentInput{
		PromptID: prompt.ID,
		UserID:   user1.ID,
		ParentID: &top.ID,
		Body:     "回复",
	})
	if err != nil {
		t.Fatalf("create reply: %v", err)
	}
	if reply.RootID == nil || *reply.RootID != top.ID {
		t.Fatalf("unexpected root id: %+v", reply.RootID)
	}
	if reply.ParentID == nil || *reply.ParentID != top.ID {
		t.Fatalf("unexpected parent id: %+v", reply.ParentID)
	}
}

func TestPromptCommentServiceList(t *testing.T) {
	service, db := setupPromptCommentService(t, promptcommentsvc.Config{
		DefaultPageSize: 5,
		MaxPageSize:     20,
		RequireApproval: false,
		MaxBodyLength:   500,
	}, nil)
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	ctx := context.Background()
	user := userdomain.User{Username: "alice", Email: "alice@example.com"}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	prompt := promptdomain.Prompt{
		UserID:           user.ID,
		Topic:            "话题",
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
	root, err := service.Create(ctx, promptcommentsvc.CreateCommentInput{
		PromptID: prompt.ID,
		UserID:   user.ID,
		Body:     "第一条",
	})
	if err != nil {
		t.Fatalf("create root: %v", err)
	}
	if _, err := service.Create(ctx, promptcommentsvc.CreateCommentInput{
		PromptID: prompt.ID,
		UserID:   user.ID,
		ParentID: &root.ID,
		Body:     "二楼",
	}); err != nil {
		t.Fatalf("create reply: %v", err)
	}
	if _, err := service.Create(ctx, promptcommentsvc.CreateCommentInput{
		PromptID: prompt.ID,
		UserID:   user.ID,
		Body:     "第二条",
	}); err != nil {
		t.Fatalf("create second root: %v", err)
	}
	result, err := service.List(ctx, promptcommentsvc.ListCommentsInput{
		PromptID: prompt.ID,
		Status:   promptdomain.PromptCommentStatusApproved,
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("list comments: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("unexpected thread count: %d", len(result.Items))
	}
	if result.Items[0].Root.ReplyCount != 1 {
		t.Fatalf("unexpected reply count: %d", result.Items[0].Root.ReplyCount)
	}
}

func TestPromptCommentServiceCreateAuditTriggered(t *testing.T) {
	var called bool
	audit := func(_ context.Context, _ uint, content string) error {
		called = true
		if content != "审核测试" {
			t.Fatalf("unexpected audit content: %s", content)
		}
		return nil
	}
	service, db := setupPromptCommentService(t, promptcommentsvc.Config{
		DefaultPageSize: 5,
		MaxPageSize:     20,
		RequireApproval: false,
		MaxBodyLength:   500,
	}, audit)
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	ctx := context.Background()
	user := userdomain.User{Username: "tester", Email: "tester@example.com"}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	prompt := promptdomain.Prompt{
		UserID:           user.ID,
		Topic:            "审核主题",
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
	if _, err := service.Create(ctx, promptcommentsvc.CreateCommentInput{
		PromptID: prompt.ID,
		UserID:   user.ID,
		Body:     "审核测试",
	}); err != nil {
		t.Fatalf("create comment: %v", err)
	}
	if !called {
		t.Fatalf("expected audit to be triggered")
	}
}

func TestPromptCommentServiceCreateAuditReject(t *testing.T) {
	auditErr := errors.New("审核不通过")
	audit := func(_ context.Context, _ uint, _ string) error {
		return auditErr
	}
	service, db := setupPromptCommentService(t, promptcommentsvc.Config{
		DefaultPageSize: 5,
		MaxPageSize:     20,
		RequireApproval: false,
		MaxBodyLength:   500,
	}, audit)
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	ctx := context.Background()
	user := userdomain.User{Username: "reject", Email: "reject@example.com"}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	prompt := promptdomain.Prompt{
		UserID:           user.ID,
		Topic:            "审核主题",
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
	if _, err := service.Create(ctx, promptcommentsvc.CreateCommentInput{
		PromptID: prompt.ID,
		UserID:   user.ID,
		Body:     "将被拒绝",
	}); !errors.Is(err, auditErr) {
		t.Fatalf("expected audit error, got %v", err)
	}
}

func TestPromptCommentServiceDeleteCascade(t *testing.T) {
	service, db := setupPromptCommentService(t, promptcommentsvc.Config{
		DefaultPageSize: 5,
		MaxPageSize:     20,
		RequireApproval: false,
		MaxBodyLength:   500,
	}, nil)
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	ctx := context.Background()
	user := userdomain.User{Username: "admin", Email: "admin@example.com"}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	prompt := promptdomain.Prompt{
		UserID:           user.ID,
		Topic:            "话题",
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
	root, err := service.Create(ctx, promptcommentsvc.CreateCommentInput{
		PromptID: prompt.ID,
		UserID:   user.ID,
		Body:     "根评论",
	})
	if err != nil {
		t.Fatalf("create root: %v", err)
	}
	if _, err := service.Create(ctx, promptcommentsvc.CreateCommentInput{
		PromptID: prompt.ID,
		UserID:   user.ID,
		ParentID: &root.ID,
		Body:     "楼中楼",
	}); err != nil {
		t.Fatalf("create reply: %v", err)
	}
	if err := service.Delete(ctx, root.ID); err != nil {
		t.Fatalf("delete comment: %v", err)
	}
	var count int64
	if err := db.WithContext(ctx).Model(&promptdomain.PromptComment{}).Count(&count).Error; err != nil {
		t.Fatalf("count comments: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected all comments deleted, remaining=%d", count)
	}
}
