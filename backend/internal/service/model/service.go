/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-11 00:49:56
 * @FilePath: \electron-go-app\backend\internal\service\model\service.go
 * @LastEditTime: 2025-10-13 20:37:52
 */
package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/infra/security"
	"electron-go-app/backend/internal/repository"

	"gorm.io/gorm"
)

var (
	ErrCredentialNotFound   = errors.New("model credential not found")
	ErrDuplicatedModelKey   = errors.New("model credential already exists")
	ErrInvalidStatus        = errors.New("invalid credential status")
	ErrStatusMismatchUpdate = errors.New("status update failed")
	ErrCredentialDisabled   = errors.New("model credential disabled")
	ErrUnsupportedProvider  = errors.New("unsupported model provider")
)

// supportedProviders 维护允许接入的模型提供方列表。
var supportedProviders = map[string]struct{}{
	"deepseek":   {},
	"volcengine": {},
}

// Credential 表示对外返回的模型凭据（脱敏）。
type Credential struct {
	ID             uint           `json:"id"`
	Provider       string         `json:"provider"`
	ModelKey       string         `json:"model_key"`
	DisplayName    string         `json:"display_name"`
	BaseURL        string         `json:"base_url"`
	ExtraConfig    map[string]any `json:"extra_config"`
	Status         string         `json:"status"`
	LastVerifiedAt *time.Time     `json:"last_verified_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// CreateInput 描述新增模型凭据所需的字段。
type CreateInput struct {
	Provider    string
	ModelKey    string
	DisplayName string
	BaseURL     string
	APIKey      string
	ExtraConfig map[string]any
}

// UpdateInput 定义可更新的凭据信息。
type UpdateInput struct {
	DisplayName *string
	BaseURL     *string
	APIKey      *string
	ExtraConfig map[string]any
	Status      *string
}

// ResolveProviderByModelKey 返回指定模型 key 对应凭据的 provider。
func (s *Service) ResolveProviderByModelKey(ctx context.Context, userID uint, modelKey string) (string, error) {
	credential, err := s.repo.FindByModelKey(ctx, userID, modelKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrCredentialNotFound
		}
		return "", fmt.Errorf("find credential: %w", err)
	}
	return normalizeProvider(credential.Provider), nil
}

// Service 聚合模型凭据仓储与用户仓储，用于跨层更新偏好设置。
type Service struct {
	repo  *repository.ModelCredentialRepository
	users *repository.UserRepository
}

// NewService 构造模型凭据服务。
func NewService(repo *repository.ModelCredentialRepository, users *repository.UserRepository) *Service {
	return &Service{repo: repo, users: users}
}

// List 返回用户所有模型凭据（脱敏）。
func (s *Service) List(ctx context.Context, userID uint) ([]Credential, error) {
	entities, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	credentials := make([]Credential, 0, len(entities))
	for _, entity := range entities {
		cred, err := toCredential(entity)
		if err != nil {
			return nil, err
		}
		credentials = append(credentials, cred)
	}
	return credentials, nil
}

// Create 新增模型凭据并返回脱敏信息。
func (s *Service) Create(ctx context.Context, userID uint, input CreateInput) (Credential, error) {
	if err := s.validateCreateInput(ctx, userID, input); err != nil {
		return Credential{}, err
	}
	sealed, err := encryptAPIKey(input.APIKey)
	if err != nil {
		return Credential{}, err
	}
	extraJSON, err := encodeExtraConfig(input.ExtraConfig)
	if err != nil {
		return Credential{}, err
	}
	provider := normalizeProvider(input.Provider)
	modelKey := strings.TrimSpace(input.ModelKey)
	entity := domain.UserModelCredential{
		UserID:       userID,
		Provider:     provider,
		ModelKey:     modelKey,
		DisplayName:  strings.TrimSpace(input.DisplayName),
		BaseURL:      strings.TrimSpace(input.BaseURL),
		APIKeyCipher: sealed,
		ExtraConfig:  extraJSON,
		Status:       "enabled",
	}
	if err := s.repo.Create(ctx, &entity); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return Credential{}, ErrDuplicatedModelKey
		}
		return Credential{}, fmt.Errorf("create credential: %w", err)
	}
	return toCredential(entity)
}

// Update 修改凭据属性（含重新加密 APIKey）。
func (s *Service) Update(ctx context.Context, userID, id uint, input UpdateInput) (Credential, error) {
	entity, err := s.repo.FindByID(ctx, userID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Credential{}, ErrCredentialNotFound
		}
		return Credential{}, fmt.Errorf("find credential: %w", err)
	}
	previousStatus := entity.Status

	if input.DisplayName != nil {
		entity.DisplayName = strings.TrimSpace(*input.DisplayName)
	}
	if input.BaseURL != nil {
		entity.BaseURL = strings.TrimSpace(*input.BaseURL)
	}
	if input.APIKey != nil {
		sealed, err := encryptAPIKey(*input.APIKey)
		if err != nil {
			return Credential{}, err
		}
		entity.APIKeyCipher = sealed
	}
	if input.ExtraConfig != nil {
		extraJSON, err := encodeExtraConfig(input.ExtraConfig)
		if err != nil {
			return Credential{}, err
		}
		entity.ExtraConfig = extraJSON
	}
	if input.Status != nil {
		nextStatus, err := normalizeStatus(*input.Status)
		if err != nil {
			return Credential{}, err
		}
		entity.Status = nextStatus
	}

	if err := s.repo.Update(ctx, entity); err != nil {
		return Credential{}, fmt.Errorf("update credential: %w", err)
	}
	if previousStatus != entity.Status && entity.Status == "disabled" {
		// 禁用后即刻撤销用户偏好，避免前端拿到失效模型。
		if err := s.clearPreferredModelIfMatched(ctx, entity.UserID, entity.ModelKey); err != nil {
			return Credential{}, fmt.Errorf("clear preferred model: %w", err)
		}
	}
	return toCredential(*entity)
}

// Delete 移除凭据。
func (s *Service) Delete(ctx context.Context, userID, id uint) error {
	entity, err := s.repo.FindByID(ctx, userID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCredentialNotFound
		}
		return fmt.Errorf("find credential: %w", err)
	}

	if err := s.repo.Delete(ctx, userID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCredentialNotFound
		}
		return fmt.Errorf("delete credential: %w", err)
	}

	// 删除与禁用共用同一偏好清理逻辑。
	if err := s.clearPreferredModelIfMatched(ctx, entity.UserID, entity.ModelKey); err != nil {
		return fmt.Errorf("clear preferred model: %w", err)
	}
	return nil
}

// validateCreateInput 校验新增模型凭据所需字段是否完整且合法。
func (s *Service) validateCreateInput(ctx context.Context, userID uint, input CreateInput) error {
	provider := normalizeProvider(input.Provider)
	if provider == "" {
		return errors.New("provider is required")
	}
	if !isSupportedProvider(provider) {
		return ErrUnsupportedProvider
	}
	modelKey := strings.TrimSpace(input.ModelKey)
	if modelKey == "" {
		return errors.New("model_key is required")
	}
	if strings.TrimSpace(input.DisplayName) == "" {
		return errors.New("display_name is required")
	}
	if strings.TrimSpace(input.APIKey) == "" {
		return errors.New("api_key is required")
	}
	exists, err := s.repo.ExistsWithModelKey(ctx, userID, modelKey, nil)
	if err != nil {
		return fmt.Errorf("check duplicate: %w", err)
	}
	if exists {
		return ErrDuplicatedModelKey
	}
	return nil
}

// encryptAPIKey 对模型 API Key 进行加密存储。
func encryptAPIKey(key string) ([]byte, error) {
	sealed, err := security.Encrypt([]byte(strings.TrimSpace(key)))
	if err != nil {
		return nil, fmt.Errorf("encrypt api key: %w", err)
	}
	return sealed, nil
}

// encodeExtraConfig 将扩展配置序列化为 JSON 字符串。
func encodeExtraConfig(extra map[string]any) (string, error) {
	if extra == nil {
		return "{}", nil
	}
	data, err := json.Marshal(extra)
	if err != nil {
		return "", fmt.Errorf("encode extra config: %w", err)
	}
	return string(data), nil
}

// toCredential 将数据库实体转换为对外返回的脱敏结构。
func toCredential(entity domain.UserModelCredential) (Credential, error) {
	extra := map[string]any{}
	if entity.ExtraConfig != "" {
		if err := json.Unmarshal([]byte(entity.ExtraConfig), &extra); err != nil {
			return Credential{}, fmt.Errorf("decode extra config: %w", err)
		}
	}
	return Credential{
		ID:             entity.ID,
		Provider:       entity.Provider,
		ModelKey:       entity.ModelKey,
		DisplayName:    entity.DisplayName,
		BaseURL:        entity.BaseURL,
		ExtraConfig:    extra,
		Status:         entity.Status,
		LastVerifiedAt: entity.LastVerifiedAt,
		CreatedAt:      entity.CreatedAt,
		UpdatedAt:      entity.UpdatedAt,
	}, nil
}

// normalizeProvider 统一 provider 大小写与空白处理，便于比较。
func normalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

// isSupportedProvider 判断 provider 是否在白名单内。
func isSupportedProvider(provider string) bool {
	_, ok := supportedProviders[normalizeProvider(provider)]
	return ok
}

// normalizeStatus 校验并规范凭据的状态字段。
func normalizeStatus(status string) (string, error) {
	trimmed := strings.TrimSpace(strings.ToLower(status))
	if trimmed == "" {
		return "", ErrInvalidStatus
	}
	switch trimmed {
	case "enabled", "disabled":
		return trimmed, nil
	default:
		return "", ErrInvalidStatus
	}
}

// clearPreferredModelIfMatched 在凭据被禁用/删除时重置用户偏好。
func (s *Service) clearPreferredModelIfMatched(ctx context.Context, userID uint, modelKey string) error {
	if s.users == nil {
		return nil
	}
	// 重新读取用户设置，若偏好命中则回退到默认值。
	userEntity, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("find user: %w", err)
	}
	settings, err := domain.ParseSettings(userEntity.Settings)
	if err != nil {
		return fmt.Errorf("parse settings: %w", err)
	}
	if settings.PreferredModel != modelKey {
		return nil
	}
	settings.PreferredModel = domain.DefaultSettings().PreferredModel
	raw, err := domain.SettingsJSON(settings)
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	if err := s.users.UpdateSettings(ctx, userID, raw); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("update settings: %w", err)
	}
	return nil
}
