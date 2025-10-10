/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 20:39:17
 * @FilePath: \electron-go-app\backend\internal\repository\user_repository.go
 * @LastEditTime: 2025-10-08 20:39:22
 */
package repository

import (
	"context"
	"strings"
	"time"

	"electron-go-app/backend/internal/domain/user"

	"gorm.io/gorm"
)

// UserRepository 封装用户相关的数据访问方法，基于 GORM 实现。
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户仓储实例，接收共享的 *gorm.DB。
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create 写入用户记录。
func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	return r.db.WithContext(ctx).Create(u).Error
}

// FindByID 根据主键查找用户。
func (r *UserRepository) FindByID(ctx context.Context, id uint) (*user.User, error) {
	var u user.User
	if err := r.db.WithContext(ctx).First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// FindByEmail 通过邮箱查找用户，若不存在返回 gorm.ErrRecordNotFound。
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	var u user.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// FindByUsername 通过用户名查找用户。
func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*user.User, error) {
	var u user.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// Update 按主键更新用户信息。
func (r *UserRepository) Update(ctx context.Context, u *user.User) error {
	return r.db.WithContext(ctx).Save(u).Error
}

// MarkEmailVerified 将用户邮箱标记为已验证。
func (r *UserRepository) MarkEmailVerified(ctx context.Context, userID uint, when time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&user.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"email_verified_at": when,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateSettings 更新指定用户的设置字段。
func (r *UserRepository) UpdateSettings(ctx context.Context, userID uint, settings string) error {
	result := r.db.WithContext(ctx).
		Model(&user.User{}).
		Where("id = ?", userID).
		Update("settings", settings)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateProfileFields 按需更新用户的基础信息字段。
func (r *UserRepository) UpdateProfileFields(ctx context.Context, userID uint, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}

	// 预处理字段：避免空字符串写入数据库导致违反非空约束或出现无意义数据。
	for key, value := range fields {
		if str, ok := value.(string); ok {
			trimmed := strings.TrimSpace(str)
			if key == "avatar_url" {
				// 头像允许清空，因此仅同步去除首尾空格。
				fields[key] = trimmed
				continue
			}
			if trimmed == "" {
				delete(fields, key)
			} else if trimmed != str {
				fields[key] = trimmed
			}
		}
	}

	if len(fields) == 0 {
		return nil
	}

	result := r.db.WithContext(ctx).
		Model(&user.User{}).
		Where("id = ?", userID).
		Updates(fields)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ExistsOtherByEmail 检查是否存在除指定用户外使用该邮箱的记录。
func (r *UserRepository) ExistsOtherByEmail(ctx context.Context, email string, excludeID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&user.User{}).
		Where("email = ? AND id <> ?", email, excludeID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ExistsOtherByUsername 检查是否存在除指定用户外使用该用户名的记录。
func (r *UserRepository) ExistsOtherByUsername(ctx context.Context, username string, excludeID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&user.User{}).
		Where("username = ? AND id <> ?", username, excludeID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
