package repository

import (
	"context"
	"errors"
	"fmt"

	promptdomain "electron-go-app/backend/internal/domain/prompt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PromptRepository 负责 Prompt 及其版本记录的持久化操作。
type PromptRepository struct {
	db *gorm.DB
}

// NewPromptRepository 创建 PromptRepository。
func NewPromptRepository(db *gorm.DB) *PromptRepository {
	return &PromptRepository{db: db}
}

// Create 新增 Prompt 记录。
func (r *PromptRepository) Create(ctx context.Context, entity *promptdomain.Prompt) error {
	if entity == nil {
		return errors.New("prompt entity is nil")
	}
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		return fmt.Errorf("create prompt: %w", err)
	}
	return nil
}

// Update 更新 Prompt 记录。
func (r *PromptRepository) Update(ctx context.Context, entity *promptdomain.Prompt) error {
	if entity == nil {
		return errors.New("prompt entity is nil")
	}
	if err := r.db.WithContext(ctx).Save(entity).Error; err != nil {
		return fmt.Errorf("update prompt: %w", err)
	}
	return nil
}

// FindByID 根据 ID 查询 Prompt，限定为指定用户所有。
func (r *PromptRepository) FindByID(ctx context.Context, userID, id uint) (*promptdomain.Prompt, error) {
	var entity promptdomain.Prompt
	if err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&entity).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

// FindByUserAndTopic 根据用户与 Topic 查找 Prompt，常用于处理工作区失效后的兜底逻辑。
func (r *PromptRepository) FindByUserAndTopic(ctx context.Context, userID uint, topic string) (*promptdomain.Prompt, error) {
	var entity promptdomain.Prompt
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND topic = ?", userID, topic).
		Order("updated_at DESC").
		First(&entity).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

// CreateVersion 记录 Prompt 的历史版本。
func (r *PromptRepository) CreateVersion(ctx context.Context, version *promptdomain.PromptVersion) error {
	if version == nil {
		return errors.New("prompt version is nil")
	}
	if err := r.db.WithContext(ctx).Create(version).Error; err != nil {
		return fmt.Errorf("create prompt version: %w", err)
	}
	return nil
}

// ListVersions 按版本号倒序返回指定 Prompt 的历史版本。
func (r *PromptRepository) ListVersions(ctx context.Context, promptID uint, limit int) ([]promptdomain.PromptVersion, error) {
	query := r.db.WithContext(ctx).Where("prompt_id = ?", promptID).Order("version_no DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var versions []promptdomain.PromptVersion
	if err := query.Find(&versions).Error; err != nil {
		return nil, fmt.Errorf("list prompt versions: %w", err)
	}
	return versions, nil
}

// DeleteOldVersions 超出保留数量的旧版本会被删除，避免无限增长。
func (r *PromptRepository) DeleteOldVersions(ctx context.Context, promptID uint, keep int) error {
	if keep <= 0 {
		return nil
	}
	subQuery := r.db.WithContext(ctx).
		Model(&promptdomain.PromptVersion{}).
		Select("id").
		Where("prompt_id = ?", promptID).
		Order("version_no DESC").
		Offset(keep)
	return r.db.WithContext(ctx).Where("id IN (?)", subQuery).Delete(&promptdomain.PromptVersion{}).Error
}

// KeywordRepository 负责关键词的增删查改。
type KeywordRepository struct {
	db *gorm.DB
}

// NewKeywordRepository 创建 KeywordRepository。
func NewKeywordRepository(db *gorm.DB) *KeywordRepository {
	return &KeywordRepository{db: db}
}

// Upsert 在用户+主题范围内查找关键词，若不存在则创建；存在时更新极性/来源等信息。
func (r *KeywordRepository) Upsert(ctx context.Context, entity *promptdomain.Keyword) (*promptdomain.Keyword, error) {
	if entity == nil {
		return nil, errors.New("keyword entity is nil")
	}
	var stored promptdomain.Keyword
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND topic = ? AND word = ?", entity.UserID, entity.Topic, entity.Word).
		Take(&stored).Error
	if err == nil {
		stored.Polarity = entity.Polarity
		if entity.Source != "" {
			stored.Source = entity.Source
		}
		if entity.Language != "" {
			stored.Language = entity.Language
		}
		if entity.Weight != 0 {
			stored.Weight = entity.Weight
		}
		if saveErr := r.db.WithContext(ctx).Save(&stored).Error; saveErr != nil {
			return nil, fmt.Errorf("update keyword: %w", saveErr)
		}
		return &stored, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find keyword: %w", err)
	}
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		return nil, fmt.Errorf("create keyword: %w", err)
	}
	return entity, nil
}

// Update 更新关键词属性。
func (r *KeywordRepository) Update(ctx context.Context, entity *promptdomain.Keyword) error {
	if entity == nil {
		return errors.New("keyword entity is nil")
	}
	if err := r.db.WithContext(ctx).Save(entity).Error; err != nil {
		return fmt.Errorf("update keyword: %w", err)
	}
	return nil
}

// ListByTopic 返回指定用户在 Topic 下的全部关键词。
func (r *KeywordRepository) ListByTopic(ctx context.Context, userID uint, topic string) ([]promptdomain.Keyword, error) {
	var keywords []promptdomain.Keyword
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND topic = ?", userID, topic).
		Order("polarity ASC, weight DESC, word ASC").
		Find(&keywords).Error; err != nil {
		return nil, fmt.Errorf("list keywords: %w", err)
	}
	return keywords, nil
}

// AttachToPrompt 建立 Prompt 与关键词之间的关联。
func (r *KeywordRepository) AttachToPrompt(ctx context.Context, promptID, keywordID uint, relation string) error {
	entity := promptdomain.PromptKeyword{
		PromptID:  promptID,
		KeywordID: keywordID,
		Relation:  relation,
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "prompt_id"}, {Name: "keyword_id"}},
			DoUpdates: clause.Assignments(map[string]any{"relation": relation}),
		}).
		Create(&entity).Error; err != nil {
		return fmt.Errorf("attach keyword to prompt: %w", err)
	}
	return nil
}

// DetachFromPrompt 解除 Prompt 与关键词之间的关联。
func (r *KeywordRepository) DetachFromPrompt(ctx context.Context, promptID, keywordID uint) error {
	if err := r.db.WithContext(ctx).
		Where("prompt_id = ? AND keyword_id = ?", promptID, keywordID).
		Delete(&promptdomain.PromptKeyword{}).Error; err != nil {
		return fmt.Errorf("detach keyword: %w", err)
	}
	return nil
}

// ListPromptKeywords 返回 Prompt 关联的关键词列表。
func (r *KeywordRepository) ListPromptKeywords(ctx context.Context, promptID uint) ([]promptdomain.PromptKeyword, error) {
	var relations []promptdomain.PromptKeyword
	if err := r.db.WithContext(ctx).
		Where("prompt_id = ?", promptID).
		Find(&relations).Error; err != nil {
		return nil, fmt.Errorf("list prompt keywords: %w", err)
	}
	return relations, nil
}

// ReplacePromptKeywords 先清理 prompt 现有关联，再批量插入新的关联关系。
func (r *KeywordRepository) ReplacePromptKeywords(ctx context.Context, promptID uint, entries []promptdomain.PromptKeyword) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("prompt_id = ?", promptID).Delete(&promptdomain.PromptKeyword{}).Error; err != nil {
			return fmt.Errorf("delete prompt keywords: %w", err)
		}
		if len(entries) == 0 {
			return nil
		}
		if err := tx.Create(&entries).Error; err != nil {
			return fmt.Errorf("insert prompt keywords: %w", err)
		}
		return nil
	})
}
