/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 22:37:41
 * @FilePath: \electron-go-app\backend\internal\service\user\service.go
 * @LastEditTime: 2025-10-08 22:37:45
 */
package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	domain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/repository"

	"gorm.io/gorm"
)

// Service 负责用户资料的查询与更新。
type Service struct {
	users  *repository.UserRepository
	models *repository.ModelCredentialRepository
}

// NewService 构造用户服务层实例。
func NewService(users *repository.UserRepository, models *repository.ModelCredentialRepository) *Service {
	return &Service{users: users, models: models}
}

// ErrUserNotFound 表示请求的用户不存在。
var ErrUserNotFound = errors.New("user not found")

// ErrEmailTaken 表示邮箱已被其他用户占用。
var ErrEmailTaken = errors.New("email already in use")

// ErrUsernameTaken 表示用户名已被其他用户占用。
var ErrUsernameTaken = errors.New("username already in use")

// Profile 封装返回的用户资料与设置。
type Profile struct {
	User     *domain.User    `json:"user"`
	Settings domain.Settings `json:"settings"`
}

// UpdateProfileParams 封装可以更新的基础信息字段。
type UpdateProfileParams struct {
	Username  *string
	Email     *string
	AvatarURL *string
}

// GetProfile 返回指定用户的资料与设置。
func (s *Service) GetProfile(ctx context.Context, userID uint) (Profile, error) {
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Profile{}, ErrUserNotFound
		}
		return Profile{}, fmt.Errorf("find user: %w", err)
	}

	settings, err := domain.ParseSettings(u.Settings)
	if err != nil {
		return Profile{}, fmt.Errorf("parse settings: %w", err)
	}

	return Profile{User: u, Settings: settings}, nil
}

// UpdateSettings 更新用户设置并返回最新资料。
func (s *Service) UpdateSettings(ctx context.Context, userID uint, settings domain.Settings) (Profile, error) {
	if err := s.ensurePreferredModel(ctx, userID, settings.PreferredModel); err != nil {
		return Profile{}, err
	}
	raw, err := domain.SettingsJSON(settings)
	if err != nil {
		return Profile{}, fmt.Errorf("encode settings: %w", err)
	}

	if err := s.users.UpdateSettings(ctx, userID, raw); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Profile{}, ErrUserNotFound
		}
		return Profile{}, fmt.Errorf("update settings: %w", err)
	}

	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Profile{}, ErrUserNotFound
		}
		return Profile{}, fmt.Errorf("reload user: %w", err)
	}

	u.Settings = raw
	parsedSettings, err := domain.ParseSettings(raw)
	if err != nil {
		return Profile{}, fmt.Errorf("parse updated settings: %w", err)
	}
	return Profile{User: u, Settings: parsedSettings}, nil
}

// UpdateProfile 更新用户的基础资料（用户名、邮箱、头像等）。
func (s *Service) UpdateProfile(ctx context.Context, userID uint, params UpdateProfileParams) (Profile, error) {
	updates := map[string]interface{}{}

	if params.Username != nil {
		username := strings.TrimSpace(*params.Username)
		if username != "" {
			exists, err := s.users.ExistsOtherByUsername(ctx, username, userID)
			if err != nil {
				return Profile{}, fmt.Errorf("check username: %w", err)
			}
			if exists {
				return Profile{}, ErrUsernameTaken
			}
			updates["username"] = username
		}
	}

	if params.Email != nil {
		email := strings.TrimSpace(*params.Email)
		if email != "" {
			exists, err := s.users.ExistsOtherByEmail(ctx, email, userID)
			if err != nil {
				return Profile{}, fmt.Errorf("check email: %w", err)
			}
			if exists {
				return Profile{}, ErrEmailTaken
			}
			updates["email"] = email
		}
	}

	if params.AvatarURL != nil {
		avatar := strings.TrimSpace(*params.AvatarURL)
		updates["avatar_url"] = avatar
	}

	if err := s.users.UpdateProfileFields(ctx, userID, updates); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Profile{}, ErrUserNotFound
		}
		return Profile{}, fmt.Errorf("update profile: %w", err)
	}

	return s.GetProfile(ctx, userID)
}

// ErrPreferredModelNotFound 表示偏好模型不存在。
var ErrPreferredModelNotFound = errors.New("preferred model not found")

// ErrPreferredModelDisabled 表示偏好模型处于禁用状态。
var ErrPreferredModelDisabled = errors.New("preferred model disabled")

func (s *Service) ensurePreferredModel(ctx context.Context, userID uint, modelKey string) error {
	if s.models == nil {
		return nil
	}
	trimmed := strings.TrimSpace(modelKey)
	if trimmed == "" {
		return nil
	}
	defaultKey := domain.DefaultSettings().PreferredModel
	if trimmed == defaultKey {
		return nil
	}
	// 仅允许用户选择自己名下且启用的模型。
	credential, err := s.models.FindByModelKey(ctx, userID, trimmed)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPreferredModelNotFound
		}
		return fmt.Errorf("find preferred model: %w", err)
	}
	if !strings.EqualFold(credential.Status, "enabled") {
		return ErrPreferredModelDisabled
	}
	return nil
}
