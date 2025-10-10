/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 16:04:09
 * @FilePath: \electron-go-app\backend\internal\repository\email_verification_repository.go
 * @LastEditTime: 2025-10-10 16:04:13
 */
package repository

import (
	"context"
	"time"

	domain "electron-go-app/backend/internal/domain/user"

	"gorm.io/gorm"
)

// EmailVerificationRepository 管理邮箱验证令牌的存取。
type EmailVerificationRepository struct {
	db *gorm.DB
}

// NewEmailVerificationRepository 构造实例。
func NewEmailVerificationRepository(db *gorm.DB) *EmailVerificationRepository {
	return &EmailVerificationRepository{db: db}
}

// UpsertToken 为指定用户写入新的验证令牌，并覆盖旧记录。
func (r *EmailVerificationRepository) UpsertToken(ctx context.Context, token *domain.EmailVerificationToken) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", token.UserID).Delete(&domain.EmailVerificationToken{}).Error; err != nil {
			return err
		}
		if err := tx.Create(token).Error; err != nil {
			return err
		}
		return nil
	})
}

// FindValidToken 根据 token 查询有效记录（未过期、未使用）。
func (r *EmailVerificationRepository) FindValidToken(ctx context.Context, token string) (*domain.EmailVerificationToken, error) {
	var rec domain.EmailVerificationToken
	err := r.db.WithContext(ctx).
		Where("token = ? AND consumed_at IS NULL AND expires_at > ?", token, time.Now()).
		First(&rec).Error
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// MarkConsumed 标记 token 已被使用。
func (r *EmailVerificationRepository) MarkConsumed(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&domain.EmailVerificationToken{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"consumed_at": time.Now(),
		}).Error
}

// DeleteExpired 清理过期的令牌记录，返回删除的条数。
func (r *EmailVerificationRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("expires_at <= ?", before).
		Delete(&domain.EmailVerificationToken{})
	return result.RowsAffected, result.Error
}
