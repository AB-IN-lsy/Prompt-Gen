package unit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	promptdomain "electron-go-app/backend/internal/domain/prompt"
	deepseek "electron-go-app/backend/internal/infra/model/deepseek"
	"electron-go-app/backend/internal/repository"
	modelsvc "electron-go-app/backend/internal/service/model"
	promptsvc "electron-go-app/backend/internal/service/prompt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// fakeModelInvoker 用于在单元测试中模拟大模型响应。
type fakeModelInvoker struct {
	responses []deepseek.ChatCompletionResponse
	err       error
	requests  []deepseek.ChatCompletionRequest
}

func (f *fakeModelInvoker) InvokeChatCompletion(_ context.Context, _ uint, _ string, req deepseek.ChatCompletionRequest) (deepseek.ChatCompletionResponse, error) {
	f.requests = append(f.requests, req)
	if len(f.responses) == 0 {
		return deepseek.ChatCompletionResponse{}, f.err
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

type fakeWorkspaceStore struct {
	snapshot promptdomain.WorkspaceSnapshot
}

func (f *fakeWorkspaceStore) CreateOrReplace(context.Context, uint, promptdomain.WorkspaceSnapshot) (string, error) {
	return "", nil
}

func (f *fakeWorkspaceStore) MergeKeywords(context.Context, uint, string, []promptdomain.WorkspaceKeyword) error {
	return nil
}

func (f *fakeWorkspaceStore) UpdateDraftBody(context.Context, uint, string, string) error {
	return nil
}

func (f *fakeWorkspaceStore) Touch(context.Context, uint, string) error {
	return nil
}

func (f *fakeWorkspaceStore) Snapshot(context.Context, uint, string) (promptdomain.WorkspaceSnapshot, error) {
	return f.snapshot, nil
}

func (f *fakeWorkspaceStore) Delete(context.Context, uint, string) error {
	return nil
}

func (f *fakeWorkspaceStore) SetPromptMeta(context.Context, uint, string, uint, string) error {
	return nil
}

func (f *fakeWorkspaceStore) GetPromptMeta(context.Context, uint, string) (uint, string, error) {
	return 0, "", nil
}

func (f *fakeWorkspaceStore) RemoveKeyword(context.Context, uint, string, string, string) error {
	return nil
}

func (f *fakeWorkspaceStore) SetAttributes(context.Context, uint, string, map[string]string) error {
	return nil
}

func setupPromptService(t *testing.T) (*promptsvc.Service, *repository.PromptRepository, *repository.KeywordRepository, *gorm.DB, *fakeModelInvoker) {
	t.Helper()
	defaultCfg := promptsvc.Config{
		KeywordLimit:        promptsvc.DefaultKeywordLimit,
		KeywordMaxLength:    promptsvc.DefaultKeywordMaxLength,
		TagLimit:            promptsvc.DefaultTagLimit,
		TagMaxLength:        promptsvc.DefaultTagMaxLength,
		DefaultListPageSize: 20,
		MaxListPageSize:     100,
		VersionRetention:    promptsvc.DefaultVersionRetentionLimit,
	}
	return setupPromptServiceWithConfig(t, defaultCfg)
}

func setupPromptServiceWithConfig(t *testing.T, cfg promptsvc.Config) (*promptsvc.Service, *repository.PromptRepository, *repository.KeywordRepository, *gorm.DB, *fakeModelInvoker) {
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

	if err := db.AutoMigrate(&promptdomain.Prompt{}, &promptdomain.Keyword{}, &promptdomain.PromptKeyword{}, &promptdomain.PromptLike{}, &promptdomain.PromptVersion{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	promptRepo := repository.NewPromptRepository(db)
	keywordRepo := repository.NewKeywordRepository(db)
	modelStub := &fakeModelInvoker{}
	service, err := promptsvc.NewServiceWithConfig(
		promptRepo,
		keywordRepo,
		modelStub,
		nil,
		nil,
		nil,
		nil,
		nil,
		cfg,
	)
	if err != nil {
		t.Fatalf("init prompt service: %v", err)
	}
	return service, promptRepo, keywordRepo, db, modelStub
}

func buildAuditResponse(t *testing.T, allowed bool, reason string) deepseek.ChatCompletionResponse {
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

// TestPromptServiceInterpret 验证自然语言解析会落地关键词并正确去重。
func TestPromptServiceInterpret(t *testing.T) {
	service, _, keywordRepo, db, modelStub := setupPromptService(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	payload := map[string]any{
		"topic": "React 前端面试",
		"positive_keywords": []map[string]any{
			{"word": "React", "weight": 5},
			{"word": "Hooks", "weight": 4},
			{"word": "状态管理", "weight": 3},
			{"word": "React", "weight": 5},
		},
		"negative_keywords": []map[string]any{
			{"word": "过时框架", "weight": 4},
			{"word": "重复", "weight": 2},
		},
		"confidence":   0.92,
		"instructions": "强调输出结构化问答",
		"tags":         []string{"React", "面试", "React", "备考"},
	}
	content, _ := json.Marshal(payload)
	modelStub.responses = []deepseek.ChatCompletionResponse{
		buildAuditResponse(t, true, ""),
		{
			Model: "deepseek-chat",
			Choices: []deepseek.ChatCompletionChoice{
				{Message: deepseek.ChatMessage{Role: "assistant", Content: string(content)}},
			},
		},
	}

	result, err := service.Interpret(context.Background(), promptsvc.InterpretInput{
		UserID:      1,
		Description: "帮我准备一份 React 前端面试的问答提示词，别出现过时框架",
		ModelKey:    "deepseek-chat",
		Language:    "中文",
	})
	if err != nil {
		t.Fatalf("Interpret returned error: %v", err)
	}
	if result.Topic != "React 前端面试" {
		t.Fatalf("unexpected topic: %s", result.Topic)
	}
	if len(result.PositiveKeywords) != 3 {
		t.Fatalf("expected 3 positive keywords, got %d", len(result.PositiveKeywords))
	}
	if len(result.NegativeKeywords) != 2 {
		t.Fatalf("expected 2 negative keywords, got %d", len(result.NegativeKeywords))
	}
	if result.Instructions != "强调输出结构化问答" {
		t.Fatalf("unexpected instructions: %s", result.Instructions)
	}
	if len(result.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(result.Tags))
	}
	if len(modelStub.requests) != 2 {
		t.Fatalf("expected 2 model requests, got %d", len(modelStub.requests))
	}

	// 关键词应已写入数据库，方便后续补全。
	stored, err := keywordRepo.ListByTopic(context.Background(), 1, "React 前端面试")
	if err != nil {
		t.Fatalf("list keywords failed: %v", err)
	}
	if len(stored) != 5 {
		t.Fatalf("expected 5 stored keywords, got %d", len(stored))
	}
}

// TestPromptServiceInterpretInstructionsArray 捕捉到线上解析失败后新增：
// DeepSeek 偶尔会把 instructions 输出成字符串数组而非纯字符串，这里确保解析器能容错。
func TestPromptServiceInterpretInstructionsArray(t *testing.T) {
	service, _, _, db, modelStub := setupPromptService(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	payload := map[string]any{
		"topic": "Node.js 面试",
		"positive_keywords": []map[string]any{
			{"word": "并发", "weight": 5},
			{"word": "事件循环", "weight": 5},
		},
		"negative_keywords": []map[string]any{},
		"confidence":        0.8,
		"instructions":      []string{"回答时附带示例代码", "重点解释事件循环机制"},
		"tags": []any{
			"Node.js",
			"后端",
			123,
		},
	}
	content, _ := json.Marshal(payload)
	modelStub.responses = []deepseek.ChatCompletionResponse{
		buildAuditResponse(t, true, ""),
		{
			Model: "deepseek-chat",
			Choices: []deepseek.ChatCompletionChoice{
				{Message: deepseek.ChatMessage{Role: "assistant", Content: string(content)}},
			},
		},
	}

	result, err := service.Interpret(context.Background(), promptsvc.InterpretInput{
		UserID:      2,
		Description: "如何准备 Node.js 面试？",
		ModelKey:    "deepseek-chat",
		Language:    "中文",
	})
	if err != nil {
		t.Fatalf("Interpret returned error: %v", err)
	}
	if result.Topic != "Node.js 面试" {
		t.Fatalf("unexpected topic: %s", result.Topic)
	}
	if got := result.Instructions; got != "回答时附带示例代码；重点解释事件循环机制" {
		t.Fatalf("unexpected instructions: %q", got)
	}
	if len(result.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(result.Tags))
	}
	for _, tag := range result.Tags {
		if len([]rune(tag)) > promptsvc.DefaultTagMaxLength {
			t.Fatalf("tag exceeds max length: %q", tag)
		}
	}
	if len(modelStub.requests) != 2 {
		t.Fatalf("expected 2 model requests, got %d", len(modelStub.requests))
	}
}

// TestPromptServiceExportPrompts 验证导出接口会生成本地文件并包含完整 Prompt 记录。
func TestPromptServiceExportPrompts(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := promptsvc.Config{
		KeywordLimit:        promptsvc.DefaultKeywordLimit,
		KeywordMaxLength:    promptsvc.DefaultKeywordMaxLength,
		TagLimit:            promptsvc.DefaultTagLimit,
		TagMaxLength:        promptsvc.DefaultTagMaxLength,
		DefaultListPageSize: 20,
		MaxListPageSize:     100,
		ExportDirectory:     tmpDir,
	}
	service, promptRepo, _, db, _ := setupPromptServiceWithConfig(t, cfg)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	positiveJSON, err := json.Marshal([]promptdomain.PromptKeywordItem{
		{
			Word:   "示例关键词",
			Source: promptdomain.KeywordSourceManual,
			Weight: 3,
		},
	})
	if err != nil {
		t.Fatalf("marshal positive keywords: %v", err)
	}
	negativeJSON, err := json.Marshal([]promptdomain.PromptKeywordItem{
		{
			Word:   "无关词",
			Source: promptdomain.KeywordSourceManual,
			Weight: 1,
		},
	})
	if err != nil {
		t.Fatalf("marshal negative keywords: %v", err)
	}
	tagJSON, err := json.Marshal([]string{"导出"})
	if err != nil {
		t.Fatalf("marshal tags: %v", err)
	}

	prompt := promptdomain.Prompt{
		UserID:           1,
		Topic:            "导出功能自测",
		Body:             "正文内容",
		Instructions:     "附加说明",
		PositiveKeywords: string(positiveJSON),
		NegativeKeywords: string(negativeJSON),
		Model:            "deepseek-chat",
		Status:           promptdomain.PromptStatusDraft,
		Tags:             string(tagJSON),
		IsFavorited:      true,
	}
	if err := promptRepo.Create(context.Background(), &prompt); err != nil {
		t.Fatalf("create prompt: %v", err)
	}

	output, err := service.ExportPrompts(context.Background(), promptsvc.ExportPromptsInput{
		UserID: 1,
	})
	if err != nil {
		t.Fatalf("export prompts: %v", err)
	}
	if output.PromptCount != 1 {
		t.Fatalf("expected prompt count 1, got %d", output.PromptCount)
	}
	if output.FilePath == "" {
		t.Fatalf("expected non-empty file path")
	}

	absTmp, err := filepath.Abs(tmpDir)
	if err != nil {
		t.Fatalf("abs tmp dir: %v", err)
	}
	rel, err := filepath.Rel(absTmp, output.FilePath)
	if err != nil {
		t.Fatalf("relative path: %v", err)
	}
	if strings.HasPrefix(rel, "..") {
		t.Fatalf("expected export file under %s, got %s", absTmp, output.FilePath)
	}

	info, err := os.Stat(output.FilePath)
	if err != nil {
		t.Fatalf("stat export file: %v", err)
	}
	if info.IsDir() {
		t.Fatalf("expected export file, got directory")
	}

	data, err := os.ReadFile(output.FilePath)
	if err != nil {
		t.Fatalf("read export file: %v", err)
	}

	var exported struct {
		PromptCount int `json:"prompt_count"`
		Prompts     []struct {
			Topic            string `json:"topic"`
			IsFavorited      bool   `json:"is_favorited"`
			PositiveKeywords []struct {
				Word string `json:"word"`
			} `json:"positive_keywords"`
		} `json:"prompts"`
	}
	if err := json.Unmarshal(data, &exported); err != nil {
		t.Fatalf("unmarshal export file: %v", err)
	}
	if exported.PromptCount != 1 {
		t.Fatalf("unexpected prompt_count: %d", exported.PromptCount)
	}
	if len(exported.Prompts) != 1 {
		t.Fatalf("unexpected prompt size: %d", len(exported.Prompts))
	}
	if exported.Prompts[0].Topic != "导出功能自测" {
		t.Fatalf("unexpected topic: %s", exported.Prompts[0].Topic)
	}
	if len(exported.Prompts[0].PositiveKeywords) != 1 {
		t.Fatalf("unexpected keyword length: %d", len(exported.Prompts[0].PositiveKeywords))
	}
	if exported.Prompts[0].PositiveKeywords[0].Word != "示例关键词" {
		t.Fatalf("unexpected keyword word: %s", exported.Prompts[0].PositiveKeywords[0].Word)
	}
	if !exported.Prompts[0].IsFavorited {
		t.Fatalf("expected prompt favorited flag true")
	}
}

func TestPromptServiceListPromptVersions(t *testing.T) {
	service, promptRepo, _, db, _ := setupPromptService(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	prompt := promptdomain.Prompt{
		UserID:           1,
		Topic:            "版本测试",
		Body:             "正文 v1",
		Instructions:     "说明 v1",
		PositiveKeywords: `[ {"word":"示例","weight":3} ]`,
		NegativeKeywords: `[]`,
		Model:            "deepseek-chat",
		Status:           promptdomain.PromptStatusPublished,
		Tags:             "[]",
		LatestVersionNo:  2,
	}
	if err := promptRepo.Create(context.Background(), &prompt); err != nil {
		t.Fatalf("create prompt: %v", err)
	}

	versions := []promptdomain.PromptVersion{
		{
			PromptID:         prompt.ID,
			VersionNo:        1,
			Body:             "正文 v1",
			Instructions:     "说明 v1",
			PositiveKeywords: `[ {"word":"示例","weight":3} ]`,
			NegativeKeywords: `[]`,
			Model:            "deepseek-chat",
		},
		{
			PromptID:         prompt.ID,
			VersionNo:        2,
			Body:             "正文 v2",
			Instructions:     "说明 v2",
			PositiveKeywords: `[ {"word":"升级","weight":4} ]`,
			NegativeKeywords: `[]`,
			Model:            "deepseek-chat",
		},
	}
	for _, version := range versions {
		if err := promptRepo.CreateVersion(context.Background(), &version); err != nil {
			t.Fatalf("create version: %v", err)
		}
	}

	out, err := service.ListPromptVersions(context.Background(), promptsvc.ListVersionsInput{
		UserID:   1,
		PromptID: prompt.ID,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("ListPromptVersions: %v", err)
	}
	if len(out.Versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(out.Versions))
	}
	if out.Versions[0].VersionNo != 2 {
		t.Fatalf("expected latest version first, got %d", out.Versions[0].VersionNo)
	}
}

func TestPromptServiceUpdateFavorite(t *testing.T) {
	service, promptRepo, _, db, _ := setupPromptService(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	res, err := service.Save(ctx, promptsvc.SaveInput{
		UserID:           1,
		Topic:            "收藏测试",
		Body:             "正文",
		Instructions:     "说明",
		Model:            "deepseek-chat",
		Status:           promptdomain.PromptStatusDraft,
		Publish:          false,
		Tags:             []string{"收藏"},
		PositiveKeywords: []promptsvc.KeywordItem{{Word: "示例", Polarity: promptdomain.KeywordPolarityPositive, Weight: 3}},
		NegativeKeywords: []promptsvc.KeywordItem{},
	})
	if err != nil {
		t.Fatalf("seed prompt: %v", err)
	}

	if err := service.UpdateFavorite(ctx, promptsvc.UpdateFavoriteInput{
		UserID:    1,
		PromptID:  res.PromptID,
		Favorited: true,
	}); err != nil {
		t.Fatalf("set favorite: %v", err)
	}

	stored, err := promptRepo.FindByID(ctx, 1, res.PromptID)
	if err != nil {
		t.Fatalf("load prompt: %v", err)
	}
	if !stored.IsFavorited {
		t.Fatalf("expected prompt favorited flag true")
	}

	list, err := service.ListPrompts(ctx, promptsvc.ListPromptsInput{
		UserID:        1,
		FavoritedOnly: true,
	})
	if err != nil {
		t.Fatalf("list prompts: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != res.PromptID {
		t.Fatalf("unexpected list result: %+v", list.Items)
	}

	if err := service.UpdateFavorite(ctx, promptsvc.UpdateFavoriteInput{
		UserID:    1,
		PromptID:  res.PromptID,
		Favorited: false,
	}); err != nil {
		t.Fatalf("unset favorite: %v", err)
	}
	stored, err = promptRepo.FindByID(ctx, 1, res.PromptID)
	if err != nil {
		t.Fatalf("reload prompt: %v", err)
	}
	if stored.IsFavorited {
		t.Fatalf("expected prompt favorited flag false")
	}
}

func TestPromptServiceLikeToggle(t *testing.T) {
	service, promptRepo, _, db, _ := setupPromptService(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	prompt := promptdomain.Prompt{
		UserID:           1,
		Topic:            "点赞测试",
		Body:             "内容",
		Instructions:     "说明",
		PositiveKeywords: `[{"word":"正向","weight":5}]`,
		NegativeKeywords: `[]`,
		Model:            "deepseek-chat",
		Status:           promptdomain.PromptStatusDraft,
		Tags:             "[]",
	}
	if err := promptRepo.Create(ctx, &prompt); err != nil {
		t.Fatalf("create prompt: %v", err)
	}

	result, err := service.LikePrompt(ctx, promptsvc.UpdateLikeInput{
		UserID:   1,
		PromptID: prompt.ID,
	})
	if err != nil {
		t.Fatalf("like prompt: %v", err)
	}
	if !result.Liked || result.LikeCount != 1 {
		t.Fatalf("unexpected like result: %+v", result)
	}

	reloaded, err := promptRepo.FindByID(ctx, 1, prompt.ID)
	if err != nil {
		t.Fatalf("reload prompt: %v", err)
	}
	if reloaded.LikeCount != 1 || !reloaded.IsLiked {
		t.Fatalf("unexpected prompt state after like: %+v", reloaded)
	}

	// 重复点赞不会重复计数。
	result, err = service.LikePrompt(ctx, promptsvc.UpdateLikeInput{
		UserID:   1,
		PromptID: prompt.ID,
	})
	if err != nil {
		t.Fatalf("duplicate like prompt: %v", err)
	}
	if result.LikeCount != 1 || !result.Liked {
		t.Fatalf("duplicate like should keep count 1, got %+v", result)
	}

	list, err := service.ListPrompts(ctx, promptsvc.ListPromptsInput{
		UserID: 1,
	})
	if err != nil {
		t.Fatalf("list prompts: %v", err)
	}
	if len(list.Items) != 1 || !list.Items[0].IsLiked || list.Items[0].LikeCount != 1 {
		t.Fatalf("unexpected list entry: %+v", list.Items)
	}

	result, err = service.UnlikePrompt(ctx, promptsvc.UpdateLikeInput{
		UserID:   1,
		PromptID: prompt.ID,
	})
	if err != nil {
		t.Fatalf("unlike prompt: %v", err)
	}
	if result.Liked || result.LikeCount != 0 {
		t.Fatalf("unexpected unlike result: %+v", result)
	}

	// 重复取消应保持 0。
	result, err = service.UnlikePrompt(ctx, promptsvc.UpdateLikeInput{
		UserID:   1,
		PromptID: prompt.ID,
	})
	if err != nil {
		t.Fatalf("duplicate unlike prompt: %v", err)
	}
	if result.LikeCount != 0 || result.Liked {
		t.Fatalf("duplicate unlike should keep count 0, got %+v", result)
	}
}

func TestPromptServiceGetPromptVersionDetail(t *testing.T) {
	service, promptRepo, _, db, _ := setupPromptService(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	prompt := promptdomain.Prompt{
		UserID:           1,
		Topic:            "回溯测试",
		Body:             "最新正文",
		Instructions:     "最新说明",
		PositiveKeywords: `[ {"word":"当前","weight":5} ]`,
		NegativeKeywords: `[]`,
		Model:            "deepseek-chat",
		Status:           promptdomain.PromptStatusPublished,
		Tags:             "[]",
		LatestVersionNo:  2,
	}
	if err := promptRepo.Create(context.Background(), &prompt); err != nil {
		t.Fatalf("create prompt: %v", err)
	}

	version := promptdomain.PromptVersion{
		PromptID:         prompt.ID,
		VersionNo:        2,
		Body:             "历史正文",
		Instructions:     "历史说明",
		PositiveKeywords: `[ {"word":"历史","weight":4} ]`,
		NegativeKeywords: `[ {"word":"弃用","weight":1} ]`,
		Model:            "deepseek-chat",
	}
	if err := promptRepo.CreateVersion(context.Background(), &version); err != nil {
		t.Fatalf("create version: %v", err)
	}

	detail, err := service.GetPromptVersionDetail(context.Background(), promptsvc.GetVersionDetailInput{
		UserID:    1,
		PromptID:  prompt.ID,
		VersionNo: 2,
	})
	if err != nil {
		t.Fatalf("GetPromptVersionDetail: %v", err)
	}
	if detail.Body != "历史正文" {
		t.Fatalf("unexpected body: %s", detail.Body)
	}
	if len(detail.PositiveKeywords) != 1 || detail.PositiveKeywords[0].Word != "历史" {
		t.Fatalf("unexpected positive keywords: %+v", detail.PositiveKeywords)
	}
	if len(detail.NegativeKeywords) != 1 || detail.NegativeKeywords[0].Word != "弃用" {
		t.Fatalf("unexpected negative keywords: %+v", detail.NegativeKeywords)
	}
}

// TestPromptServiceManualKeywordDuplicate 验证重复关键词的工作区去重逻辑，保证前端不再出现“删了仍提示上限”的问题。
func TestPromptServiceManualKeywordDuplicate(t *testing.T) {
	_, promptRepo, keywordRepo, db, modelStub := setupPromptService(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()
	_ = modelStub

	store := &fakeWorkspaceStore{
		snapshot: promptdomain.WorkspaceSnapshot{
			Positive: []promptdomain.WorkspaceKeyword{
				{Word: "React", Polarity: promptdomain.KeywordPolarityPositive},
			},
		},
	}
	serviceWithWorkspace, err := promptsvc.NewServiceWithConfig(
		promptRepo,
		keywordRepo,
		modelStub,
		store,
		nil,
		nil,
		nil,
		nil,
		promptsvc.Config{
			KeywordLimit:        promptsvc.DefaultKeywordLimit,
			KeywordMaxLength:    promptsvc.DefaultKeywordMaxLength,
			TagLimit:            promptsvc.DefaultTagLimit,
			TagMaxLength:        promptsvc.DefaultTagMaxLength,
			DefaultListPageSize: 20,
			MaxListPageSize:     100,
		},
	)
	if err != nil {
		t.Fatalf("init prompt service with workspace: %v", err)
	}

	_, err = serviceWithWorkspace.AddManualKeyword(context.Background(), promptsvc.ManualKeywordInput{
		UserID:         1,
		Topic:          "React",
		Word:           "React",
		Polarity:       "positive",
		WorkspaceToken: "token",
	})
	if !errors.Is(err, promptsvc.ErrDuplicateKeyword) {
		t.Fatalf("expected duplicate keyword error, got %v", err)
	}
}

// TestPromptServiceAugmentKeywords 验证模型补充关键词时会自动跳过重复项。
func TestPromptServiceAugmentKeywords(t *testing.T) {
	service, _, _, db, modelStub := setupPromptService(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	payload := map[string]any{
		"positive_keywords": []map[string]any{
			{"word": "性能优化", "weight": 5},
			{"word": "状态管理", "weight": 3},
		},
		"negative_keywords": []map[string]any{
			{"word": "重复", "weight": 2},
		},
	}
	content, _ := json.Marshal(payload)
	modelStub.responses = []deepseek.ChatCompletionResponse{
		{
			Model: "deepseek-chat",
			Choices: []deepseek.ChatCompletionChoice{
				{Message: deepseek.ChatMessage{Role: "assistant", Content: string(content)}},
			},
		},
	}

	out, err := service.AugmentKeywords(context.Background(), promptsvc.AugmentInput{
		UserID:           1,
		Topic:            "React 前端面试",
		ModelKey:         "deepseek-chat",
		ExistingPositive: []promptsvc.KeywordItem{{Word: "状态管理", Polarity: promptdomain.KeywordPolarityPositive, Weight: 3}},
		ExistingNegative: []promptsvc.KeywordItem{},
	})
	if err != nil {
		t.Fatalf("AugmentKeywords error: %v", err)
	}
	if len(out.Positive) != 1 {
		t.Fatalf("expected 1 positive keyword after dedupe, got %d", len(out.Positive))
	}
	if out.Positive[0].Word != "性能优化" {
		t.Fatalf("unexpected positive keyword: %+v", out.Positive[0])
	}
	if out.Positive[0].Weight != 5 {
		t.Fatalf("unexpected weight for positive keyword: %+v", out.Positive[0])
	}
	if len(out.Negative) != 1 || out.Negative[0].Word != "重复" {
		t.Fatalf("unexpected negative keywords: %+v", out.Negative)
	}
}

// TestPromptServiceGeneratePrompt 验证生成流程会返回模型的正文。
func TestPromptServiceGeneratePrompt(t *testing.T) {
	service, _, _, db, modelStub := setupPromptService(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	modelStub.responses = []deepseek.ChatCompletionResponse{
		{
			Model: "deepseek-chat",
			Choices: []deepseek.ChatCompletionChoice{
				{Message: deepseek.ChatMessage{Role: "assistant", Content: "这是准备 React 技术面试的 Prompt 正文"}},
			},
			Usage: &deepseek.ChatCompletionUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		},
		buildAuditResponse(t, true, ""),
	}

	start := time.Now()
	out, err := service.GeneratePrompt(context.Background(), promptsvc.GenerateInput{
		UserID:           1,
		Topic:            "React 面试",
		ModelKey:         "deepseek-chat",
		PositiveKeywords: []promptsvc.KeywordItem{{Word: "React"}, {Word: "Hooks"}},
		NegativeKeywords: []promptsvc.KeywordItem{{Word: "陈旧方案"}},
	})
	if err != nil {
		t.Fatalf("GeneratePrompt error: %v", err)
	}
	if out.Prompt == "" {
		t.Fatalf("expected non-empty prompt content")
	}
	if out.Duration <= 0 || out.Duration > time.Since(start) {
		t.Fatalf("unexpected duration: %v", out.Duration)
	}
	if out.Usage == nil || out.Usage.TotalTokens != 30 {
		t.Fatalf("unexpected usage: %+v", out.Usage)
	}
	if len(modelStub.requests) != 2 {
		t.Fatalf("expected 2 model requests, got %d", len(modelStub.requests))
	}
}

// TestPromptServiceInterpretAuditReject 验证内容审核未通过时会直接中断解析流程。
func TestPromptServiceInterpretAuditReject(t *testing.T) {
	service, _, _, db, modelStub := setupPromptService(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	modelStub.responses = []deepseek.ChatCompletionResponse{
		buildAuditResponse(t, false, "描述包含违规内容"),
	}

	_, err := service.Interpret(context.Background(), promptsvc.InterpretInput{
		UserID:      1,
		Description: "这里包含敏感词，需要被拦截",
		ModelKey:    "deepseek-chat",
		Language:    "中文",
	})
	if err == nil {
		t.Fatalf("expected error but got nil")
	}
	if !errors.Is(err, promptsvc.ErrContentRejected) {
		t.Fatalf("expected ErrContentRejected, got %v", err)
	}
	if len(modelStub.requests) != 1 {
		t.Fatalf("expected 1 model request, got %d", len(modelStub.requests))
	}
}

// TestPromptServiceGeneratePromptAuditReject 验证生成后的 Prompt 审核未通过会返回错误。
func TestPromptServiceGeneratePromptAuditReject(t *testing.T) {
	service, _, _, db, modelStub := setupPromptService(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	modelStub.responses = []deepseek.ChatCompletionResponse{
		{
			Model: "deepseek-chat",
			Choices: []deepseek.ChatCompletionChoice{
				{Message: deepseek.ChatMessage{Role: "assistant", Content: "这是违规 Prompt"}},
			},
		},
		buildAuditResponse(t, false, "生成内容涉及违禁信息"),
	}

	_, err := service.GeneratePrompt(context.Background(), promptsvc.GenerateInput{
		UserID:           1,
		Topic:            "违规主题",
		ModelKey:         "deepseek-chat",
		PositiveKeywords: []promptsvc.KeywordItem{{Word: "合法"}},
	})
	if err == nil {
		t.Fatalf("expected error but got nil")
	}
	if !errors.Is(err, promptsvc.ErrContentRejected) {
		t.Fatalf("expected ErrContentRejected, got %v", err)
	}
	if len(modelStub.requests) != 2 {
		t.Fatalf("expected 2 model requests, got %d", len(modelStub.requests))
	}
}

// TestPromptServiceSave 验证保存草稿与发布版本的行为。
func TestPromptServiceSave(t *testing.T) {
	service, promptRepo, _, db, _ := setupPromptService(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	saveDraft, err := service.Save(context.Background(), promptsvc.SaveInput{
		UserID:           1,
		Topic:            "React 面试",
		Body:             "Draft Prompt Body",
		Model:            "deepseek-chat",
		Status:           promptdomain.PromptStatusDraft,
		PositiveKeywords: []promptsvc.KeywordItem{{Word: "React"}},
		NegativeKeywords: []promptsvc.KeywordItem{},
	})
	if err != nil {
		t.Fatalf("save draft error: %v", err)
	}
	if saveDraft.Status != promptdomain.PromptStatusDraft {
		t.Fatalf("expected draft status, got %s", saveDraft.Status)
	}
	if saveDraft.PromptID == 0 {
		t.Fatalf("expected prompt id generated")
	}

	savePublish, err := service.Save(context.Background(), promptsvc.SaveInput{
		UserID:           1,
		PromptID:         saveDraft.PromptID,
		Topic:            "React 面试",
		Body:             "Published Prompt Body",
		Model:            "deepseek-chat",
		Publish:          true,
		PositiveKeywords: []promptsvc.KeywordItem{{Word: "React"}},
		NegativeKeywords: []promptsvc.KeywordItem{},
	})
	if err != nil {
		t.Fatalf("publish error: %v", err)
	}
	if savePublish.Status != promptdomain.PromptStatusPublished {
		t.Fatalf("expected published status, got %s", savePublish.Status)
	}
	if savePublish.Version != 1 {
		t.Fatalf("expected version 1, got %d", savePublish.Version)
	}

	versions, err := promptRepo.ListVersions(context.Background(), saveDraft.PromptID, 10)
	if err != nil {
		t.Fatalf("list versions error: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version stored, got %d", len(versions))
	}
	if versions[0].Body != "Published Prompt Body" {
		t.Fatalf("unexpected version body: %s", versions[0].Body)
	}
}

// TestPromptServiceSavePublishValidation 验证发布时缺少必填字段会返回明确错误，而草稿存储仍可接受不完整内容。
func TestPromptServiceSavePublishValidation(t *testing.T) {
	service, _, _, db, _ := setupPromptService(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	// 发布时缺少补充要求、负向关键词与标签，应返回错误。
	_, err := service.Save(ctx, promptsvc.SaveInput{
		UserID:                   1,
		Topic:                    "发布校验",
		Body:                     "生成内容",
		Model:                    "deepseek-chat",
		Publish:                  true,
		PositiveKeywords:         []promptsvc.KeywordItem{{Word: "React"}},
		NegativeKeywords:         nil,
		Tags:                     nil,
		EnforcePublishValidation: true,
	})
	if err == nil || !strings.Contains(err.Error(), "发布失败") {
		t.Fatalf("expected publish validation error, got %v", err)
	}

	// 保存草稿时允许字段缺失。
	draft, err := service.Save(ctx, promptsvc.SaveInput{
		UserID:           1,
		Topic:            "",
		Body:             "",
		Model:            "",
		Publish:          false,
		Status:           promptdomain.PromptStatusDraft,
		PositiveKeywords: []promptsvc.KeywordItem{},
		NegativeKeywords: []promptsvc.KeywordItem{},
		Tags:             []string{},
	})
	if err != nil {
		t.Fatalf("unexpected draft error: %v", err)
	}
	if draft.Status != promptdomain.PromptStatusDraft {
		t.Fatalf("draft status mismatch: %s", draft.Status)
	}

	// 补全必要字段再次发布应成功。
	_, err = service.Save(ctx, promptsvc.SaveInput{
		UserID:                   1,
		PromptID:                 draft.PromptID,
		Topic:                    "发布校验",
		Body:                     "生成内容",
		Instructions:             "请使用 STAR 框架",
		Model:                    "deepseek-chat",
		Publish:                  true,
		PositiveKeywords:         []promptsvc.KeywordItem{{Word: "React"}},
		NegativeKeywords:         []promptsvc.KeywordItem{{Word: "过时框架"}},
		Tags:                     []string{"面试"},
		EnforcePublishValidation: true,
	})
	if err != nil {
		t.Fatalf("expected publish success, got %v", err)
	}
}

// TestPromptServiceImportPromptsMerge 验证导入默认采用合并模式并正确更新/新增 Prompt。
func TestPromptServiceImportPromptsMerge(t *testing.T) {
	service, promptRepo, keywordRepo, db, _ := setupPromptService(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	existing, err := service.Save(ctx, promptsvc.SaveInput{
		UserID:           1,
		Topic:            "React 面试",
		Body:             "旧正文",
		Instructions:     "旧说明",
		Model:            "deepseek-chat",
		Status:           promptdomain.PromptStatusDraft,
		Publish:          false,
		Tags:             []string{"旧标签"},
		PositiveKeywords: []promptsvc.KeywordItem{{Word: "Hooks", Polarity: promptdomain.KeywordPolarityPositive, Weight: 4}},
		NegativeKeywords: []promptsvc.KeywordItem{{Word: "过时框架", Polarity: promptdomain.KeywordPolarityNegative, Weight: 2}},
	})
	if err != nil {
		t.Fatalf("seed prompt: %v", err)
	}

	publishedAt := time.Date(2025, 10, 16, 9, 30, 0, 0, time.UTC)
	createdAt := publishedAt.Add(-2 * time.Hour)
	updatedAt := publishedAt.Add(30 * time.Minute)

	payload := map[string]any{
		"generated_at": time.Now().UTC(),
		"prompt_count": 2,
		"prompts": []any{
			map[string]any{
				"id":           existing.PromptID,
				"topic":        "React 面试",
				"body":         "更新后的正文",
				"instructions": "请突出 Hooks 与并发特性",
				"model":        "deepseek-chat",
				"status":       "published",
				"is_favorited": true,
				"tags":         []string{"合并", "更新"},
				"positive_keywords": []map[string]any{
					{"word": "Hooks", "weight": 5, "source": "manual"},
					{"word": "并发", "weight": 4, "source": "manual"},
				},
				"negative_keywords": []map[string]any{
					{"word": "过时框架", "weight": 1, "source": "manual"},
				},
				"published_at":      &publishedAt,
				"created_at":        createdAt,
				"updated_at":        updatedAt,
				"latest_version_no": 3,
			},
			map[string]any{
				"id":           0,
				"topic":        "新建 Prompt",
				"body":         "新的草稿正文",
				"instructions": "强调结构化输出",
				"model":        "deepseek-chat",
				"status":       "draft",
				"is_favorited": false,
				"tags":         []string{"新增"},
				"positive_keywords": []map[string]any{
					{"word": "总结", "weight": 4, "source": "manual"},
				},
				"negative_keywords": []map[string]any{},
				"created_at":        updatedAt,
				"updated_at":        updatedAt,
				"latest_version_no": 0,
			},
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result, err := service.ImportPrompts(ctx, promptsvc.ImportPromptsInput{UserID: 1, Payload: raw})
	if err != nil {
		t.Fatalf("import prompts: %v", err)
	}
	if result.Imported != 2 || result.Skipped != 0 || len(result.Errors) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}

	updatedPrompt, err := promptRepo.FindByID(ctx, 1, existing.PromptID)
	if err != nil {
		t.Fatalf("load updated prompt: %v", err)
	}
	if updatedPrompt.Body != "更新后的正文" {
		t.Fatalf("unexpected body: %s", updatedPrompt.Body)
	}
	if updatedPrompt.LatestVersionNo != 3 {
		t.Fatalf("expected latest version 3, got %d", updatedPrompt.LatestVersionNo)
	}
	if updatedPrompt.PublishedAt == nil || !updatedPrompt.PublishedAt.Equal(publishedAt) {
		t.Fatalf("unexpected published_at: %v", updatedPrompt.PublishedAt)
	}
	if !updatedPrompt.CreatedAt.Equal(createdAt) || !updatedPrompt.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("unexpected timestamps: created=%v updated=%v", updatedPrompt.CreatedAt, updatedPrompt.UpdatedAt)
	}
	if !updatedPrompt.IsFavorited {
		t.Fatalf("expected updated prompt favorited flag true")
	}

	detail, err := service.GetPrompt(ctx, promptsvc.GetPromptInput{UserID: 1, PromptID: existing.PromptID})
	if err != nil {
		t.Fatalf("get prompt detail: %v", err)
	}
	if len(detail.PositiveKeywords) != 2 {
		t.Fatalf("expected 2 positive keywords, got %d", len(detail.PositiveKeywords))
	}
	if !reflect.DeepEqual(detail.Tags, []string{"合并", "更新"}) {
		t.Fatalf("unexpected tags: %+v", detail.Tags)
	}

	versions, err := promptRepo.ListVersions(ctx, existing.PromptID, 10)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 1 || versions[0].VersionNo != 3 {
		t.Fatalf("unexpected versions: %+v", versions)
	}

	newPrompt, err := promptRepo.FindByUserAndTopic(ctx, 1, "新建 Prompt")
	if err != nil {
		t.Fatalf("find new prompt: %v", err)
	}
	newDetail, err := service.GetPrompt(ctx, promptsvc.GetPromptInput{UserID: 1, PromptID: newPrompt.ID})
	if err != nil {
		t.Fatalf("get new prompt detail: %v", err)
	}
	if len(newDetail.PositiveKeywords) != 1 || newDetail.PositiveKeywords[0].Word != "总结" {
		t.Fatalf("unexpected new prompt keywords: %+v", newDetail.PositiveKeywords)
	}
	if newDetail.IsFavorited {
		t.Fatalf("expected new prompt favorited flag false")
	}
	if _, err := keywordRepo.ListByTopic(ctx, 1, "新建 Prompt"); err != nil {
		t.Fatalf("list keywords: %v", err)
	}
}

// TestPromptServiceImportPromptsOverwrite 验证覆盖模式会在导入前清空原有 Prompt。
func TestPromptServiceImportPromptsOverwrite(t *testing.T) {
	service, promptRepo, _, db, _ := setupPromptService(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	if _, err := service.Save(ctx, promptsvc.SaveInput{
		UserID:           1,
		Topic:            "旧数据",
		Body:             "旧正文",
		Model:            "deepseek-chat",
		Status:           promptdomain.PromptStatusDraft,
		Publish:          false,
		PositiveKeywords: []promptsvc.KeywordItem{{Word: "旧关键词", Polarity: promptdomain.KeywordPolarityPositive}},
	}); err != nil {
		t.Fatalf("seed prompt: %v", err)
	}

	payload := map[string]any{
		"generated_at": time.Now().UTC(),
		"prompt_count": 1,
		"prompts": []any{
			map[string]any{
				"topic":        "覆盖后的 Prompt",
				"body":         "覆盖正文",
				"instructions": "覆盖说明",
				"model":        "deepseek-chat",
				"status":       "published",
				"is_favorited": true,
				"positive_keywords": []map[string]any{
					{"word": "覆盖关键词", "weight": 5, "source": "manual"},
				},
				"negative_keywords": []map[string]any{},
				"published_at":      time.Now().UTC(),
				"created_at":        time.Now().UTC(),
				"updated_at":        time.Now().UTC(),
				"latest_version_no": 1,
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result, err := service.ImportPrompts(ctx, promptsvc.ImportPromptsInput{UserID: 1, Mode: "overwrite", Payload: raw})
	if err != nil {
		t.Fatalf("import prompts: %v", err)
	}
	if result.Imported != 1 || result.Skipped != 0 || len(result.Errors) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}

	prompts, _, err := promptRepo.ListByUser(ctx, 1, repository.PromptListFilter{})
	if err != nil {
		t.Fatalf("list prompts: %v", err)
	}
	if len(prompts) != 1 || prompts[0].Topic != "覆盖后的 Prompt" {
		t.Fatalf("unexpected prompts after overwrite: %+v", prompts)
	}
	if !prompts[0].IsFavorited {
		t.Fatalf("expected overwritten prompt favorited flag true")
	}
}

// TestPromptServiceImportPromptsInvalidPayload 验证非法条目会被跳过并返回错误列表。
func TestPromptServiceImportPromptsInvalidPayload(t *testing.T) {
	service, promptRepo, _, db, _ := setupPromptService(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	ctx := context.Background()
	payload := map[string]any{
		"generated_at": time.Now().UTC(),
		"prompt_count": 1,
		"prompts": []any{
			map[string]any{
				"topic":  "",
				"body":   "",
				"model":  "deepseek-chat",
				"status": "draft",
				"positive_keywords": []map[string]any{
					{"word": "空主题", "weight": 4, "source": "manual"},
				},
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result, err := service.ImportPrompts(ctx, promptsvc.ImportPromptsInput{UserID: 1, Payload: raw})
	if err != nil {
		t.Fatalf("import prompts: %v", err)
	}
	if result.Imported != 0 || result.Skipped != 1 || len(result.Errors) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !strings.Contains(result.Errors[0].Reason, "topic is required") {
		t.Fatalf("unexpected error reason: %s", result.Errors[0].Reason)
	}

	prompts, _, err := promptRepo.ListByUser(ctx, 1, repository.PromptListFilter{})
	if err != nil {
		t.Fatalf("list prompts: %v", err)
	}
	if len(prompts) != 0 {
		t.Fatalf("expected no prompts inserted, got %d", len(prompts))
	}
}

func TestPromptServiceSaveTagLimitExceeded(t *testing.T) {
	cfg := promptsvc.Config{
		KeywordLimit:        promptsvc.DefaultKeywordLimit,
		KeywordMaxLength:    promptsvc.DefaultKeywordMaxLength,
		TagLimit:            2,
		TagMaxLength:        promptsvc.DefaultTagMaxLength,
		DefaultListPageSize: 20,
		MaxListPageSize:     100,
	}
	service, _, _, db, _ := setupPromptServiceWithConfig(t, cfg)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	_, err := service.Save(context.Background(), promptsvc.SaveInput{
		UserID:  1,
		Topic:   "标签上限校验",
		Body:    "Prompt 内容",
		Model:   "deepseek-chat",
		Status:  promptdomain.PromptStatusDraft,
		Tags:    []string{"alpha", "beta", "gamma"},
		Publish: false,
		PositiveKeywords: []promptsvc.KeywordItem{
			{Word: "React", Polarity: promptdomain.KeywordPolarityPositive},
		},
		NegativeKeywords: []promptsvc.KeywordItem{},
	})
	if !errors.Is(err, promptsvc.ErrTagLimitExceeded) {
		t.Fatalf("expected tag limit exceeded error, got %v", err)
	}
}

func TestPromptServiceSaveTagNormalization(t *testing.T) {
	cfg := promptsvc.Config{
		KeywordLimit:        promptsvc.DefaultKeywordLimit,
		KeywordMaxLength:    promptsvc.DefaultKeywordMaxLength,
		TagLimit:            3,
		TagMaxLength:        promptsvc.DefaultTagMaxLength,
		DefaultListPageSize: 20,
		MaxListPageSize:     100,
	}
	service, promptRepo, _, db, _ := setupPromptServiceWithConfig(t, cfg)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	output, err := service.Save(context.Background(), promptsvc.SaveInput{
		UserID:  1,
		Topic:   "标签清洗",
		Body:    "Prompt 正文",
		Model:   "deepseek-chat",
		Status:  promptdomain.PromptStatusDraft,
		Tags:    []string{" AI ", "ai", "增长", "Growth"},
		Publish: false,
		PositiveKeywords: []promptsvc.KeywordItem{
			{Word: "Prompt", Polarity: promptdomain.KeywordPolarityPositive},
		},
		NegativeKeywords: []promptsvc.KeywordItem{},
	})
	if err != nil {
		t.Fatalf("save prompt failed: %v", err)
	}

	record, err := promptRepo.FindByID(context.Background(), 1, output.PromptID)
	if err != nil {
		t.Fatalf("load prompt: %v", err)
	}

	var stored []string
	if err := json.Unmarshal([]byte(record.Tags), &stored); err != nil {
		t.Fatalf("decode stored tags: %v", err)
	}

	expected := []string{"AI", "增长", "Growt"}
	if !reflect.DeepEqual(stored, expected) {
		t.Fatalf("unexpected tags: %#v, expected %#v", stored, expected)
	}
}

// TestPromptServiceSaveMixedLanguageTopicSpacing 验证中英文 Topic 在保存时会自动插入分隔符，方便全文检索命中。
func TestPromptServiceSaveMixedLanguageTopicSpacing(t *testing.T) {
	service, promptRepo, _, db, _ := setupPromptService(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	rawTopic := "Go后端工程师技术面试Prompt生成"
	output, err := service.Save(context.Background(), promptsvc.SaveInput{
		UserID:  1,
		Topic:   rawTopic,
		Body:    "Prompt 正文",
		Model:   "deepseek-chat",
		Status:  promptdomain.PromptStatusDraft,
		Publish: false,
		PositiveKeywords: []promptsvc.KeywordItem{
			{Word: "Prompt", Polarity: promptdomain.KeywordPolarityPositive},
		},
		NegativeKeywords: []promptsvc.KeywordItem{},
	})
	if err != nil {
		t.Fatalf("save prompt failed: %v", err)
	}

	record, err := promptRepo.FindByID(context.Background(), 1, output.PromptID)
	if err != nil {
		t.Fatalf("load prompt: %v", err)
	}

	expectedTopic := "Go 后端工程师技术面试 Prompt 生成"
	if record.Topic != expectedTopic {
		t.Fatalf("unexpected stored topic: %q, expected %q", record.Topic, expectedTopic)
	}

	found, err := promptRepo.FindByUserAndTopic(context.Background(), 1, expectedTopic)
	if err != nil {
		t.Fatalf("find by topic failed: %v", err)
	}
	if found.ID != output.PromptID {
		t.Fatalf("unexpected prompt id, got %d expected %d", found.ID, output.PromptID)
	}
}

func TestGeneratePromptUsesFreeTierWhenCredentialMissing(t *testing.T) {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("extract sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	defer sqlDB.Close()

	if err := db.AutoMigrate(&promptdomain.Prompt{}, &promptdomain.Keyword{}, &promptdomain.PromptKeyword{}, &promptdomain.PromptLike{}, &promptdomain.PromptVersion{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	promptRepo := repository.NewPromptRepository(db)
	keywordRepo := repository.NewKeywordRepository(db)

	primaryInvoker := &fakeModelInvoker{err: modelsvc.ErrCredentialNotFound}
	fallbackInvoker := &fakeModelInvoker{responses: []deepseek.ChatCompletionResponse{
		{
			Model: "deepseek-chat",
			Choices: []deepseek.ChatCompletionChoice{
				{Message: deepseek.ChatMessage{Role: "assistant", Content: "测试 Prompt 正文"}},
			},
		},
	}}
	auditPayload := map[string]any{
		"allowed": true,
		"reason":  "",
	}
	auditRaw, _ := json.Marshal(auditPayload)
	auditInvoker := &fakeModelInvoker{responses: []deepseek.ChatCompletionResponse{
		{
			Model: "audit-model",
			Choices: []deepseek.ChatCompletionChoice{
				{Message: deepseek.ChatMessage{Role: "assistant", Content: string(auditRaw)}},
			},
		},
	}}

	cfg := promptsvc.Config{
		KeywordLimit:        promptsvc.DefaultKeywordLimit,
		KeywordMaxLength:    promptsvc.DefaultKeywordMaxLength,
		TagLimit:            promptsvc.DefaultTagLimit,
		TagMaxLength:        promptsvc.DefaultTagMaxLength,
		DefaultListPageSize: promptsvc.DefaultPromptListPageSize,
		MaxListPageSize:     promptsvc.DefaultPromptListMaxPageSize,
		FreeTier: promptsvc.FreeTierConfig{
			Enabled:     true,
			Alias:       "",
			ActualModel: "deepseek-chat",
			DisplayName: "测试免费模型",
			DailyQuota:  10,
			Window:      time.Hour,
			Invoker:     fallbackInvoker,
		},
		Audit: promptsvc.AuditConfig{
			Enabled:  true,
			ModelKey: "audit-model",
			Invoker:  auditInvoker,
		},
	}

	service, err := promptsvc.NewServiceWithConfig(promptRepo, keywordRepo, primaryInvoker, nil, nil, nil, nil, nil, cfg)
	if err != nil {
		t.Fatalf("init prompt service: %v", err)
	}

	result, err := service.GeneratePrompt(context.Background(), promptsvc.GenerateInput{
		UserID:           1,
		Topic:            "测试主题",
		ModelKey:         "",
		PositiveKeywords: []promptsvc.KeywordItem{{Word: "React", Polarity: promptdomain.KeywordPolarityPositive, Weight: 5}},
	})
	if err != nil {
		t.Fatalf("generate prompt failed: %v", err)
	}
	if result.Prompt != "测试 Prompt 正文" {
		t.Fatalf("unexpected prompt: %s", result.Prompt)
	}
	if len(primaryInvoker.requests) != 1 {
		t.Fatalf("expected primary invoker to be called once, got %d", len(primaryInvoker.requests))
	}
	if len(fallbackInvoker.requests) != 1 {
		t.Fatalf("expected fallback invoker to be called once, got %d", len(fallbackInvoker.requests))
	}
	if fallbackInvoker.requests[0].Model != "deepseek-chat" {
		t.Fatalf("expected fallback request model deepseek-chat, got %s", fallbackInvoker.requests[0].Model)
	}
}
