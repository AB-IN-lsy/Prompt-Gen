package unit

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	promptdomain "electron-go-app/backend/internal/domain/prompt"
	deepseek "electron-go-app/backend/internal/infra/model/deepseek"
	"electron-go-app/backend/internal/repository"
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

	if err := db.AutoMigrate(&promptdomain.Prompt{}, &promptdomain.Keyword{}, &promptdomain.PromptKeyword{}, &promptdomain.PromptVersion{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	promptRepo := repository.NewPromptRepository(db)
	keywordRepo := repository.NewKeywordRepository(db)
	modelStub := &fakeModelInvoker{}
	service := promptsvc.NewServiceWithConfig(
		promptRepo,
		keywordRepo,
		modelStub,
		nil,
		nil,
		nil,
		cfg,
	)
	return service, promptRepo, keywordRepo, db, modelStub
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
	serviceWithWorkspace := promptsvc.NewServiceWithConfig(
		promptRepo,
		keywordRepo,
		modelStub,
		store,
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

	_, err := serviceWithWorkspace.AddManualKeyword(context.Background(), promptsvc.ManualKeywordInput{
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
