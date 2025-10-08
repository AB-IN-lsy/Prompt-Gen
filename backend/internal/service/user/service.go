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

	domain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/repository"

	"gorm.io/gorm"
)

// Service 负责用户资料的查询与更新。
type Service struct {
	users *repository.UserRepository
}

// NewService 构造用户服务层实例。
func NewService(users *repository.UserRepository) *Service {
	return &Service{users: users}
}

// ErrUserNotFound 表示请求的用户不存在。
var ErrUserNotFound = errors.New("user not found")

// Profile 封装返回的用户资料与设置。
type Profile struct {
	User     *domain.User    `json:"user"`
	Settings domain.Settings `json:"settings"`
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
	return Profile{User: u, Settings: settings}, nil
}
