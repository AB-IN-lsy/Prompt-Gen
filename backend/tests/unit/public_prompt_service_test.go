package unit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	promptdomain "electron-go-app/backend/internal/domain/prompt"
	"electron-go-app/backend/internal/repository"
	publicpromptsvc "electron-go-app/backend/internal/service/publicprompt"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupPublicPromptService 构建公共 Prompt 服务的测试实例。
func setupPublicPromptService(t *testing.T, allowSubmission bool) (*publicpromptsvc.Service, *repository.PublicPromptRepository, *repository.PromptRepository, *gorm.DB) {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := db.AutoMigrate(&promptdomain.PublicPrompt{}, &promptdomain.Prompt{}, &promptdomain.PromptLike{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	publicRepo := repository.NewPublicPromptRepository(db)
	promptRepo := repository.NewPromptRepository(db)
	logger := zap.NewNop().Sugar()
	service := publicpromptsvc.NewService(publicRepo, db, logger, allowSubmission)
	return service, publicRepo, promptRepo, db
}

// TestPublicPromptServiceSubmitRequiresPublishedSource 验证仅允许发布后的 Prompt 投稿。
func TestPublicPromptServiceSubmitRequiresPublishedSource(t *testing.T) {
	service, _, promptRepo, db := setupPublicPromptService(t, true)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	seed := &promptdomain.Prompt{
		UserID:           1,
		Topic:            "React 面试",
		Body:             "Prompt Body",
		Instructions:     "补充说明",
		PositiveKeywords: toJSON(t, []promptdomain.PromptKeywordItem{{Word: "React"}}),
		NegativeKeywords: toJSON(t, []promptdomain.PromptKeywordItem{{Word: "过时框架"}}),
		Model:            "deepseek-chat",
		Status:           promptdomain.PromptStatusDraft,
		Tags:             toJSON(t, []string{"面试"}),
		LatestVersionNo:  1,
	}
	if err := promptRepo.Create(ctx, seed); err != nil {
		t.Fatalf("create prompt: %v", err)
	}

	_, err := service.Submit(ctx, publicpromptsvc.SubmitInput{
		AuthorUserID:     1,
		SourcePromptID:   &seed.ID,
		Title:            "React 面试指南",
		Topic:            "React 面试",
		Summary:          "帮助候选人准备 React 面试",
		Body:             "Prompt Body",
		Instructions:     "补充说明",
		PositiveKeywords: "[]",
		NegativeKeywords: "[]",
		Tags:             "[]",
		Model:            "deepseek-chat",
		Language:         "zh-CN",
	})
	if err == nil || !strings.Contains(err.Error(), "仅发布后的 Prompt") {
		t.Fatalf("expected publish requirement error, got %v", err)
	}

	seed.Status = promptdomain.PromptStatusPublished
	seed.PublishedAt = pointerTo(time.Now())
	if err := promptRepo.Update(ctx, seed); err != nil {
		t.Fatalf("update prompt: %v", err)
	}

	entity, err := service.Submit(ctx, publicpromptsvc.SubmitInput{
		AuthorUserID:     1,
		SourcePromptID:   &seed.ID,
		Title:            "React 面试指南",
		Topic:            "React 面试",
		Summary:          "帮助候选人准备 React 面试",
		Body:             "Prompt Body",
		Instructions:     "补充说明",
		PositiveKeywords: "[]",
		NegativeKeywords: "[]",
		Tags:             "[]",
		Model:            "deepseek-chat",
		Language:         "zh-CN",
	})
	if err != nil {
		t.Fatalf("submit prompt: %v", err)
	}
	if entity.Status != promptdomain.PublicPromptStatusPending {
		t.Fatalf("unexpected status: %s", entity.Status)
	}
	if entity.SourcePromptID == nil || *entity.SourcePromptID != seed.ID {
		t.Fatalf("expected source prompt id %d, got %+v", seed.ID, entity.SourcePromptID)
	}
}

// TestPublicPromptServiceListFilters 验证列表过滤逻辑满足审批与作者筛选需求。
func TestPublicPromptServiceListFilters(t *testing.T) {
	service, repo, _, db := setupPublicPromptService(t, true)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()
	ctx := context.Background()

	fixtures := []promptdomain.PublicPrompt{
		{
			AuthorUserID:     1,
			Title:            "已通过条目",
			Topic:            "Topic A",
			Summary:          "Summary",
			Body:             "Body",
			Instructions:     "Instructions",
			PositiveKeywords: "[]",
			NegativeKeywords: "[]",
			Tags:             `["tag"]`,
			Model:            "deepseek-chat",
			Language:         "zh-CN",
			Status:           promptdomain.PublicPromptStatusApproved,
		},
		{
			AuthorUserID:     1,
			Title:            "待审核条目",
			Topic:            "Topic B",
			Summary:          "Summary",
			Body:             "Body",
			Instructions:     "Instructions",
			PositiveKeywords: "[]",
			NegativeKeywords: "[]",
			Tags:             `["tag"]`,
			Model:            "deepseek-chat",
			Language:         "zh-CN",
			Status:           promptdomain.PublicPromptStatusPending,
		},
		{
			AuthorUserID:     2,
			Title:            "他人驳回条目",
			Topic:            "Topic C",
			Summary:          "Summary",
			Body:             "Body",
			Instructions:     "Instructions",
			PositiveKeywords: "[]",
			NegativeKeywords: "[]",
			Tags:             `["tag"]`,
			Model:            "deepseek-chat",
			Language:         "zh-CN",
			Status:           promptdomain.PublicPromptStatusRejected,
		},
	}
	for i := range fixtures {
		if err := repo.Create(ctx, &fixtures[i]); err != nil {
			t.Fatalf("seed public prompt %d: %v", i, err)
		}
	}

	approved, err := service.List(ctx, publicpromptsvc.ListFilter{
		OnlyApproved: true,
		Page:         1,
		PageSize:     10,
		ViewerUserID: 1,
	})
	if err != nil {
		t.Fatalf("list approved: %v", err)
	}
	if len(approved.Items) != 1 || approved.Items[0].Status != promptdomain.PublicPromptStatusApproved {
		t.Fatalf("unexpected approved result: %+v", approved.Items)
	}

	pendingMine, err := service.List(ctx, publicpromptsvc.ListFilter{
		Status:       promptdomain.PublicPromptStatusPending,
		AuthorUserID: 1,
		Page:         1,
		PageSize:     10,
		ViewerUserID: 1,
	})
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pendingMine.Items) != 1 || pendingMine.Items[0].AuthorUserID != 1 {
		t.Fatalf("unexpected pending result: %+v", pendingMine.Items)
	}
}

// TestPublicPromptServiceDelete 验证删除后无法再次访问。
func TestPublicPromptServiceDelete(t *testing.T) {
	service, repo, _, db := setupPublicPromptService(t, true)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	record := &promptdomain.PublicPrompt{
		AuthorUserID:     1,
		Title:            "待删除条目",
		Topic:            "Topic",
		Summary:          "Summary",
		Body:             "Body",
		Instructions:     "Instructions",
		PositiveKeywords: "[]",
		NegativeKeywords: "[]",
		Tags:             "[]",
		Model:            "deepseek-chat",
		Language:         "zh-CN",
		Status:           promptdomain.PublicPromptStatusPending,
	}
	if err := repo.Create(ctx, record); err != nil {
		t.Fatalf("seed public prompt: %v", err)
	}

	if err := service.Delete(ctx, record.ID); err != nil {
		t.Fatalf("delete prompt: %v", err)
	}
	if err := service.Delete(ctx, record.ID); !errors.Is(err, publicpromptsvc.ErrPublicPromptNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

// TestPublicPromptServiceResubmitAfterReject 验证同一主题被驳回后可覆盖投稿并重置审核信息。
func TestPublicPromptServiceResubmitAfterReject(t *testing.T) {
	service, repo, promptRepo, db := setupPublicPromptService(t, true)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()
	ctx := context.Background()

	seedPrompt := &promptdomain.Prompt{
		UserID:           1,
		Topic:            "复古港风照片",
		Body:             "Prompt",
		Instructions:     "说明",
		PositiveKeywords: toJSON(t, []promptdomain.PromptKeywordItem{{Word: "复古"}}),
		NegativeKeywords: toJSON(t, []promptdomain.PromptKeywordItem{{Word: "现代"}}),
		Model:            "deepseek-chat",
		Status:           promptdomain.PromptStatusPublished,
		Tags:             toJSON(t, []string{"摄影"}),
		LatestVersionNo:  1,
		PublishedAt:      pointerTo(time.Now()),
	}
	if err := promptRepo.Create(ctx, seedPrompt); err != nil {
		t.Fatalf("create prompt: %v", err)
	}

	first, err := service.Submit(ctx, publicpromptsvc.SubmitInput{
		AuthorUserID:     1,
		SourcePromptID:   &seedPrompt.ID,
		Title:            "复古港风照片",
		Topic:            "复古港风照片",
		Summary:          "首次投稿",
		Body:             "Prompt",
		Instructions:     "说明",
		PositiveKeywords: "[]",
		NegativeKeywords: "[]",
		Tags:             "[]",
		Model:            "deepseek-chat",
		Language:         "zh-CN",
	})
	if err != nil {
		t.Fatalf("submit first: %v", err)
	}

	first.Status = promptdomain.PublicPromptStatusRejected
	reviewerID := uint(2)
	first.ReviewerUserID = &reviewerID
	first.ReviewReason = "细节不充分"
	if err := repo.Update(ctx, first); err != nil {
		t.Fatalf("update reject: %v", err)
	}

	second, err := service.Submit(ctx, publicpromptsvc.SubmitInput{
		AuthorUserID:     1,
		SourcePromptID:   &seedPrompt.ID,
		Title:            "复古港风照片 2.0",
		Topic:            "复古港风照片",
		Summary:          "重新提交",
		Body:             "新 Prompt",
		Instructions:     "更详细说明",
		PositiveKeywords: "[\"复古\"]",
		NegativeKeywords: "[\"模糊\"]",
		Tags:             "[\"摄影\"]",
		Model:            "deepseek-chat",
		Language:         "zh-CN",
	})
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected reuse existing record")
	}
	if second.Status != promptdomain.PublicPromptStatusPending {
		t.Fatalf("status not reset: %s", second.Status)
	}
	if second.ReviewReason != "" || second.ReviewerUserID != nil {
		t.Fatalf("review info not cleared: %+v", second)
	}
	if second.Title != "复古港风照片 2.0" || second.Summary != "重新提交" {
		t.Fatalf("fields not updated: %+v", second)
	}
}

func TestPublicPromptServiceLikeFlow(t *testing.T) {
	service, repo, promptRepo, db := setupPublicPromptService(t, true)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	basePrompt := promptdomain.Prompt{
		UserID:           1,
		Topic:            "公开点赞基础",
		Body:             "正文",
		Instructions:     "说明",
		PositiveKeywords: `[]`,
		NegativeKeywords: `[]`,
		Tags:             `[]`,
		Model:            "deepseek-chat",
		Status:           promptdomain.PromptStatusPublished,
		PublishedAt:      pointerTo(time.Now()),
	}
	if err := promptRepo.Create(ctx, &basePrompt); err != nil {
		t.Fatalf("create prompt: %v", err)
	}

	entry := promptdomain.PublicPrompt{
		SourcePromptID:   pointerTo(basePrompt.ID),
		AuthorUserID:     1,
		Title:            "公开条目",
		Topic:            "Topic",
		Summary:          "摘要",
		Body:             "正文",
		Instructions:     "说明",
		PositiveKeywords: `[]`,
		NegativeKeywords: `[]`,
		Tags:             `[]`,
		Model:            "deepseek-chat",
		Language:         "zh-CN",
		Status:           promptdomain.PublicPromptStatusApproved,
	}
	if err := repo.Create(ctx, &entry); err != nil {
		t.Fatalf("create public prompt: %v", err)
	}

	viewerID := uint(2)
	likeResult, err := service.Like(ctx, viewerID, entry.ID)
	if err != nil {
		t.Fatalf("like public prompt: %v", err)
	}
	if !likeResult.Liked || likeResult.LikeCount != 1 {
		t.Fatalf("unexpected like result: %+v", likeResult)
	}

	detail, err := service.Get(ctx, entry.ID, viewerID)
	if err != nil {
		t.Fatalf("get public prompt: %v", err)
	}
	if detail.LikeCount != 1 || !detail.IsLiked {
		t.Fatalf("like snapshot not populated: %+v", detail)
	}

	list, err := service.List(ctx, publicpromptsvc.ListFilter{
		OnlyApproved: true,
		Page:         1,
		PageSize:     10,
		ViewerUserID: viewerID,
	})
	if err != nil {
		t.Fatalf("list public prompts: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].LikeCount != 1 || !list.Items[0].IsLiked {
		t.Fatalf("list like snapshot mismatch: %+v", list.Items)
	}

	unlikeResult, err := service.Unlike(ctx, viewerID, entry.ID)
	if err != nil {
		t.Fatalf("unlike public prompt: %v", err)
	}
	if unlikeResult.Liked || unlikeResult.LikeCount != 0 {
		t.Fatalf("unexpected unlike result: %+v", unlikeResult)
	}
}

func toJSON[T any](t *testing.T, value T) string {
	t.Helper()
	bytes, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return string(bytes)
}

func pointerTo[T any](value T) *T {
	return &value
}
