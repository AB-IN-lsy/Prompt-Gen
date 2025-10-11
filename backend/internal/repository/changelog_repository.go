/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-12 11:10:05
 * @FilePath: \electron-go-app\backend\internal\repository\changelog_repository.go
 * @LastEditTime: 2025-10-12 11:10:05
 */
package repository

import (
	"context"

	"electron-go-app/backend/internal/domain/changelog"

	"gorm.io/gorm"
)

// ChangelogRepository 提供 changelog_entries 表的 CRUD 封装。
type ChangelogRepository struct {
	db *gorm.DB
}

// NewChangelogRepository 构造仓储实例。
func NewChangelogRepository(db *gorm.DB) *ChangelogRepository {
	return &ChangelogRepository{db: db}
}

// ListByLocale 按发布时间倒序返回指定语言的日志列表。
func (r *ChangelogRepository) ListByLocale(ctx context.Context, locale string, limit int) ([]changelog.Entry, error) {
	var entries []changelog.Entry

	query := r.db.WithContext(ctx).
		Model(&changelog.Entry{}).
		Order("published_at DESC, id DESC")

	if locale != "" {
		query = query.Where("locale = ?", locale)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

// FindByID 根据主键查找日志记录。
func (r *ChangelogRepository) FindByID(ctx context.Context, id uint) (*changelog.Entry, error) {
	var entry changelog.Entry
	if err := r.db.WithContext(ctx).First(&entry, id).Error; err != nil {
		return nil, err
	}
	return &entry, nil
}

// Create 新增日志记录。
func (r *ChangelogRepository) Create(ctx context.Context, entry *changelog.Entry) error {
	return r.db.WithContext(ctx).Create(entry).Error
}

// Update 按主键更新日志记录。
func (r *ChangelogRepository) Update(ctx context.Context, entry *changelog.Entry) error {
	return r.db.WithContext(ctx).Save(entry).Error
}

// Delete 删除指定日志。
func (r *ChangelogRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&changelog.Entry{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
