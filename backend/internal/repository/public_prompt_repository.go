package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	promptdomain "electron-go-app/backend/internal/domain/prompt"

	"gorm.io/gorm"
)

// PublicPromptListFilter 描述公共库查询所需的过滤条件。
type PublicPromptListFilter struct {
	Query        string
	Status       string
	AuthorUserID uint
	OnlyApproved bool
	Limit        int
	Offset       int
}

// PublicPromptRepository 负责公共 Prompt 库的增删查改。
type PublicPromptRepository struct {
	db *gorm.DB
}

// NewPublicPromptRepository 创建公共库仓储。
func NewPublicPromptRepository(db *gorm.DB) *PublicPromptRepository {
	return &PublicPromptRepository{db: db}
}

// WithDB 基于传入的 gorm.DB 派生新的仓储，用于事务场景。
func (r *PublicPromptRepository) WithDB(db *gorm.DB) *PublicPromptRepository {
	return NewPublicPromptRepository(db)
}

// List 返回符合条件的公共 Prompt 列表与总数。
func (r *PublicPromptRepository) List(ctx context.Context, filter PublicPromptListFilter) ([]promptdomain.PublicPrompt, int64, error) {
	query := r.db.WithContext(ctx).Model(&promptdomain.PublicPrompt{})

	if filter.OnlyApproved {
		query = query.Where("status = ?", promptdomain.PublicPromptStatusApproved)
	} else if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(filter.Status))
	}

	if filter.AuthorUserID != 0 {
		query = query.Where("author_user_id = ?", filter.AuthorUserID)
	}

	if q := strings.TrimSpace(filter.Query); q != "" {
		keyword := "%" + q + "%"
		query = query.Where("(title LIKE ? OR topic LIKE ? OR tags LIKE ?)", keyword, keyword, keyword)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count public prompts: %w", err)
	}

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	var records []promptdomain.PublicPrompt
	if err := query.Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("list public prompts: %w", err)
	}
	return records, total, nil
}

// FindByID 根据 ID 查询公共 Prompt。
func (r *PublicPromptRepository) FindByID(ctx context.Context, id uint) (*promptdomain.PublicPrompt, error) {
	var entity promptdomain.PublicPrompt
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&entity).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

// Create 新增公共库记录。
func (r *PublicPromptRepository) Create(ctx context.Context, entity *promptdomain.PublicPrompt) error {
	if entity == nil {
		return errors.New("public prompt entity is nil")
	}
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		return fmt.Errorf("create public prompt: %w", err)
	}
	return nil
}

// Update 更新公共库记录。
func (r *PublicPromptRepository) Update(ctx context.Context, entity *promptdomain.PublicPrompt) error {
	if entity == nil {
		return errors.New("public prompt entity is nil")
	}
	if err := r.db.WithContext(ctx).Save(entity).Error; err != nil {
		return fmt.Errorf("update public prompt: %w", err)
	}
	return nil
}

// IncrementDownload 下载一次时自增统计字段。
func (r *PublicPromptRepository) IncrementDownload(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).
		Model(&promptdomain.PublicPrompt{}).
		Where("id = ?", id).
		UpdateColumn("download_count", gorm.Expr("download_count + 1")).Error; err != nil {
		return fmt.Errorf("increment download count: %w", err)
	}
	return nil
}
