/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 20:39:17
 * @FilePath: \electron-go-app\backend\internal\repository\user_repository.go
 * @LastEditTime: 2025-10-08 20:39:22
 */
package repository

import (
	"context"

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
