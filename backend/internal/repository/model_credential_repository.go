/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-11 00:48:33
 * @FilePath: \electron-go-app\backend\internal\repository\model_credential_repository.go
 * @LastEditTime: 2025-10-11 00:48:39
 */
package repository

import (
	"context"

	"electron-go-app/backend/internal/domain/user"

	"gorm.io/gorm"
)

// ModelCredentialRepository 提供 user_model_credentials 表的 CRUD 操作。
type ModelCredentialRepository struct {
	db *gorm.DB
}

func NewModelCredentialRepository(db *gorm.DB) *ModelCredentialRepository {
	return &ModelCredentialRepository{db: db}
}

// FindByModelKey 根据用户与 model_key 查找凭据。
func (r *ModelCredentialRepository) FindByModelKey(ctx context.Context, userID uint, modelKey string) (*user.UserModelCredential, error) {
	var credential user.UserModelCredential
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND model_key = ?", userID, modelKey).
		First(&credential).Error
	if err != nil {
		return nil, err
	}
	return &credential, nil
}

// Create 写入新的模型凭据。
func (r *ModelCredentialRepository) Create(ctx context.Context, credential *user.UserModelCredential) error {
	return r.db.WithContext(ctx).Create(credential).Error
}

// Update 部分更新现有凭据。
func (r *ModelCredentialRepository) Update(ctx context.Context, credential *user.UserModelCredential) error {
	return r.db.WithContext(ctx).Save(credential).Error
}

// Delete 软删除凭据。
func (r *ModelCredentialRepository) Delete(ctx context.Context, userID, id uint) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&user.UserModelCredential{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// FindByID 返回指定凭据。
func (r *ModelCredentialRepository) FindByID(ctx context.Context, userID, id uint) (*user.UserModelCredential, error) {
	var credential user.UserModelCredential
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&credential).Error
	if err != nil {
		return nil, err
	}
	return &credential, nil
}

// ListByUser 列出用户的全部凭据。
func (r *ModelCredentialRepository) ListByUser(ctx context.Context, userID uint) ([]user.UserModelCredential, error) {
	var credentials []user.UserModelCredential
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&credentials).Error; err != nil {
		return nil, err
	}
	return credentials, nil
}

// ExistsWithModelKey 检查同一用户下 model_key 是否已存在。
func (r *ModelCredentialRepository) ExistsWithModelKey(ctx context.Context, userID uint, modelKey string, excludeID *uint) (bool, error) {
	query := r.db.WithContext(ctx).
		Model(&user.UserModelCredential{}).
		Where("user_id = ? AND model_key = ?", userID, modelKey)
	if excludeID != nil {
		query = query.Where("id <> ?", *excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpdateStatus 更新凭据状态。
func (r *ModelCredentialRepository) UpdateStatus(ctx context.Context, userID, id uint, status string) error {
	result := r.db.WithContext(ctx).
		Model(&user.UserModelCredential{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// WithTransaction 在事务中执行操作。
func (r *ModelCredentialRepository) WithTransaction(ctx context.Context, fn func(txRepo *ModelCredentialRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&ModelCredentialRepository{db: tx})
	})
}
