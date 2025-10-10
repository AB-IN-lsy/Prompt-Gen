/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-11 01:03:56
 * @FilePath: \electron-go-app\backend\tests\unit\model_service_test.go
 * @LastEditTime: 2025-10-11 01:04:04
 */
package unit

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	domain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/infra/security"
	"electron-go-app/backend/internal/repository"
	modelsvc "electron-go-app/backend/internal/service/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestMain(m *testing.M) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	os.Setenv("MODEL_CREDENTIAL_MASTER_KEY", base64.StdEncoding.EncodeToString(key))
	os.Exit(m.Run())
}

func newTestModelService(t *testing.T) (*modelsvc.Service, *gorm.DB, *repository.UserRepository, uint) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := db.AutoMigrate(&domain.User{}, &domain.UserModelCredential{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	user := domain.User{
		Username: "tester",
		Email:    "tester@example.com",
		Settings: "{}",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	repo := repository.NewModelCredentialRepository(db)
	userRepo := repository.NewUserRepository(db)
	svc := modelsvc.NewService(repo, userRepo)
	return svc, db, userRepo, user.ID
}

func TestModelServiceCreateAndList(t *testing.T) {
	svc, db, _, userID := newTestModelService(t)
	ctx := context.Background()

	extra := map[string]any{"default_model": "gpt-4o-mini"}
	credential, err := svc.Create(ctx, userID, modelsvc.CreateInput{
		Provider:    "openai",
		ModelKey:    "primary",
		DisplayName: "OpenAI 主账号",
		BaseURL:     "https://api.openai.com",
		APIKey:      "sk-test-001",
		ExtraConfig: extra,
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	if credential.ID == 0 {
		t.Fatalf("expected credential to have id")
	}
	if credential.DisplayName != "OpenAI 主账号" {
		t.Fatalf("unexpected display name: %s", credential.DisplayName)
	}

	list, err := svc.List(ctx, userID)
	if err != nil {
		t.Fatalf("list credentials: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected single credential, got %d", len(list))
	}
	if list[0].ExtraConfig["default_model"] != "gpt-4o-mini" {
		t.Fatalf("expected default_model in extra config")
	}

	var stored domain.UserModelCredential
	if err := db.First(&stored, credential.ID).Error; err != nil {
		t.Fatalf("query stored credential: %v", err)
	}
	if len(stored.APIKeyCipher) == 0 {
		t.Fatalf("expected encrypted api key")
	}
	plain, err := security.Decrypt(stored.APIKeyCipher)
	if err != nil {
		t.Fatalf("decrypt api key: %v", err)
	}
	if string(plain) != "sk-test-001" {
		t.Fatalf("expected decrypted api key to match, got %s", string(plain))
	}
}

func TestModelServiceCreateDuplicate(t *testing.T) {
	svc, _, _, userID := newTestModelService(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, userID, modelsvc.CreateInput{
		Provider:    "openai",
		ModelKey:    "primary",
		DisplayName: "OpenAI 主账号",
		BaseURL:     "https://api.openai.com",
		APIKey:      "sk-test-001",
		ExtraConfig: map[string]any{},
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	_, err = svc.Create(ctx, userID, modelsvc.CreateInput{
		Provider:    "openai",
		ModelKey:    "primary",
		DisplayName: "另一个账号",
		BaseURL:     "https://api.openai.com",
		APIKey:      "sk-test-002",
		ExtraConfig: map[string]any{},
	})
	if !errors.Is(err, modelsvc.ErrDuplicatedModelKey) {
		t.Fatalf("expected duplicated model key error, got %v", err)
	}
}

func TestModelServiceCreateValidation(t *testing.T) {
	svc, _, _, userID := newTestModelService(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		input   modelsvc.CreateInput
		wantErr string
	}{
		{
			name: "missing provider",
			input: modelsvc.CreateInput{
				Provider:    "",
				ModelKey:    "primary",
				DisplayName: "Account",
				BaseURL:     "https://example.com",
				APIKey:      "sk-1",
			},
			wantErr: "provider is required",
		},
		{
			name: "missing model key",
			input: modelsvc.CreateInput{
				Provider:    "openai",
				ModelKey:    "  ",
				DisplayName: "Account",
				BaseURL:     "https://example.com",
				APIKey:      "sk-2",
			},
			wantErr: "model_key is required",
		},
		{
			name: "missing display name",
			input: modelsvc.CreateInput{
				Provider:    "openai",
				ModelKey:    "primary",
				DisplayName: "",
				BaseURL:     "https://example.com",
				APIKey:      "sk-3",
			},
			wantErr: "display_name is required",
		},
		{
			name: "missing api key",
			input: modelsvc.CreateInput{
				Provider:    "openai",
				ModelKey:    "primary",
				DisplayName: "Account",
				BaseURL:     "https://example.com",
				APIKey:      " ",
			},
			wantErr: "api_key is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(ctx, userID, tc.input)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestModelServiceUpdate(t *testing.T) {
	svc, db, _, userID := newTestModelService(t)
	ctx := context.Background()

	cred, err := svc.Create(ctx, userID, modelsvc.CreateInput{
		Provider:    "openai",
		ModelKey:    "primary",
		DisplayName: "OpenAI 主账号",
		BaseURL:     "https://api.openai.com",
		APIKey:      "sk-test-001",
		ExtraConfig: map[string]any{"default_model": "gpt-4o-mini"},
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	newDisplay := "OpenAI 备用账号"
	newBaseURL := "https://proxy.example.com"
	newKey := "sk-test-002"
	status := "disabled"
	updated, err := svc.Update(ctx, userID, cred.ID, modelsvc.UpdateInput{
		DisplayName: &newDisplay,
		BaseURL:     &newBaseURL,
		APIKey:      &newKey,
		ExtraConfig: map[string]any{"region": "us-east-1"},
		Status:      &status,
	})
	if err != nil {
		t.Fatalf("update credential: %v", err)
	}

	if updated.DisplayName != newDisplay {
		t.Fatalf("expected display name updated, got %s", updated.DisplayName)
	}
	if updated.BaseURL != newBaseURL {
		t.Fatalf("expected base url updated, got %s", updated.BaseURL)
	}
	if updated.Status != status {
		t.Fatalf("expected status to be %s, got %s", status, updated.Status)
	}
	if updated.ExtraConfig["region"] != "us-east-1" {
		t.Fatalf("expected extra config region override")
	}

	var stored domain.UserModelCredential
	if err := db.First(&stored, cred.ID).Error; err != nil {
		t.Fatalf("query stored credential: %v", err)
	}
	plain, err := security.Decrypt(stored.APIKeyCipher)
	if err != nil {
		t.Fatalf("decrypt api key: %v", err)
	}
	if string(plain) != newKey {
		t.Fatalf("expected decrypted key to match new value, got %s", string(plain))
	}
	if stored.Status != status {
		t.Fatalf("expected stored status to be %s, got %s", status, stored.Status)
	}
}

func TestModelServiceDelete(t *testing.T) {
	svc, _, _, userID := newTestModelService(t)
	ctx := context.Background()

	cred, err := svc.Create(ctx, userID, modelsvc.CreateInput{
		Provider:    "openai",
		ModelKey:    "primary",
		DisplayName: "OpenAI 主账号",
		BaseURL:     "https://api.openai.com",
		APIKey:      "sk-test-001",
		ExtraConfig: map[string]any{},
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	if err := svc.Delete(ctx, userID, cred.ID); err != nil {
		t.Fatalf("delete credential: %v", err)
	}

	if err := svc.Delete(ctx, userID, cred.ID); !errors.Is(err, modelsvc.ErrCredentialNotFound) {
		t.Fatalf("expected credential not found on repeated delete, got %v", err)
	}
}

func TestModelServiceDeleteClearsPreferred(t *testing.T) {
	svc, _, userRepo, userID := newTestModelService(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, userID, modelsvc.CreateInput{
		Provider:    "openai",
		ModelKey:    "primary",
		DisplayName: "OpenAI",
		BaseURL:     "",
		APIKey:      "sk-001",
		ExtraConfig: map[string]any{},
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	settings := domain.Settings{PreferredModel: "primary", SyncEnabled: true}
	raw, _ := domain.SettingsJSON(settings)
	if err := userRepo.UpdateSettings(ctx, userID, raw); err != nil {
		t.Fatalf("assign preferred model: %v", err)
	}

	creds, err := svc.List(ctx, userID)
	if err != nil || len(creds) == 0 {
		t.Fatalf("list credentials: %v", err)
	}

	if err := svc.Delete(ctx, userID, creds[0].ID); err != nil {
		t.Fatalf("delete credential: %v", err)
	}

	profileSettings, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	parsed, err := domain.ParseSettings(profileSettings.Settings)
	if err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	if parsed.PreferredModel == "primary" {
		t.Fatalf("expected preferred model cleared, still %s", parsed.PreferredModel)
	}
}

func TestModelServiceDisableClearsPreferred(t *testing.T) {
	svc, _, userRepo, userID := newTestModelService(t)
	ctx := context.Background()

	cred, err := svc.Create(ctx, userID, modelsvc.CreateInput{
		Provider:    "openai",
		ModelKey:    "primary",
		DisplayName: "OpenAI",
		BaseURL:     "",
		APIKey:      "sk-001",
		ExtraConfig: map[string]any{},
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	settings := domain.Settings{PreferredModel: "primary", SyncEnabled: false}
	raw, _ := domain.SettingsJSON(settings)
	if err := userRepo.UpdateSettings(ctx, userID, raw); err != nil {
		t.Fatalf("assign preferred model: %v", err)
	}

	status := "disabled"
	_, err = svc.Update(ctx, userID, cred.ID, modelsvc.UpdateInput{Status: &status})
	if err != nil {
		t.Fatalf("disable credential: %v", err)
	}

	profileSettings, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	parsed, err := domain.ParseSettings(profileSettings.Settings)
	if err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	if parsed.PreferredModel == "primary" {
		t.Fatalf("expected preferred model cleared after disable")
	}
}

func TestModelServiceUpdateStatusNormalizesInput(t *testing.T) {
	svc, _, userRepo, userID := newTestModelService(t)
	ctx := context.Background()

	cred, err := svc.Create(ctx, userID, modelsvc.CreateInput{
		Provider:    "openai",
		ModelKey:    "primary",
		DisplayName: "OpenAI",
		BaseURL:     "",
		APIKey:      "sk-001",
		ExtraConfig: map[string]any{"region": "us"},
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	settings := domain.Settings{PreferredModel: "primary", SyncEnabled: true}
	raw, _ := domain.SettingsJSON(settings)
	if err := userRepo.UpdateSettings(ctx, userID, raw); err != nil {
		t.Fatalf("assign preferred model: %v", err)
	}

	status := " DISABLED "
	updated, err := svc.Update(ctx, userID, cred.ID, modelsvc.UpdateInput{Status: &status})
	if err != nil {
		t.Fatalf("update credential: %v", err)
	}
	if updated.Status != "disabled" {
		t.Fatalf("expected normalized status 'disabled', got %s", updated.Status)
	}

	profileSettings, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	parsed, err := domain.ParseSettings(profileSettings.Settings)
	if err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	if parsed.PreferredModel == "primary" {
		t.Fatalf("expected preferred model cleared after normalized disable")
	}
}

func TestModelServiceUpdatePreservesExtraWhenNil(t *testing.T) {
	svc, _, _, userID := newTestModelService(t)
	ctx := context.Background()

	cred, err := svc.Create(ctx, userID, modelsvc.CreateInput{
		Provider:    "openai",
		ModelKey:    "primary",
		DisplayName: "OpenAI",
		BaseURL:     "",
		APIKey:      "sk-001",
		ExtraConfig: map[string]any{"default_model": "gpt-4o"},
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	newName := "OpenAI Updated"
	updated, err := svc.Update(ctx, userID, cred.ID, modelsvc.UpdateInput{DisplayName: &newName})
	if err != nil {
		t.Fatalf("update credential: %v", err)
	}
	if updated.DisplayName != newName {
		t.Fatalf("expected display name updated")
	}
	if updated.ExtraConfig["default_model"] != "gpt-4o" {
		t.Fatalf("expected extra config to remain unchanged")
	}
}

func TestModelServiceRejectsInvalidStatus(t *testing.T) {
	svc, _, _, userID := newTestModelService(t)
	ctx := context.Background()

	cred, err := svc.Create(ctx, userID, modelsvc.CreateInput{
		Provider:    "openai",
		ModelKey:    "primary",
		DisplayName: "OpenAI",
		BaseURL:     "",
		APIKey:      "sk-001",
		ExtraConfig: map[string]any{},
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	status := "unknown"
	_, err = svc.Update(ctx, userID, cred.ID, modelsvc.UpdateInput{Status: &status})
	if !errors.Is(err, modelsvc.ErrInvalidStatus) {
		t.Fatalf("expected invalid status error, got %v", err)
	}
}
