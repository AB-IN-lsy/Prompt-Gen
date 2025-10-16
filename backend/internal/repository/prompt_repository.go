package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	promptdomain "electron-go-app/backend/internal/domain/prompt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PromptRepository 负责 Prompt 及其版本记录的持久化操作。
type PromptRepository struct {
	db *gorm.DB
}

// PromptListFilter 定义查询“我的 Prompt”列表时使用的过滤条件。
type PromptListFilter struct {
	Status      string
	Query       string
	UseFullText bool
	Limit       int
	Offset      int
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
	var ids []uint
	if err := r.db.WithContext(ctx).
		Model(&promptdomain.PromptVersion{}).
		Where("prompt_id = ?", promptID).
		Order("version_no DESC").
		Offset(keep).
		Pluck("id", &ids).Error; err != nil {
		return fmt.Errorf("list old prompt versions: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Delete(&promptdomain.PromptVersion{}).Error; err != nil {
		return fmt.Errorf("delete prompt versions: %w", err)
	}
	return nil
}

// ListByUser 返回指定用户的 Prompt 列表，并按照更新时间倒序排列。
func (r *PromptRepository) ListByUser(ctx context.Context, userID uint, filter PromptListFilter) ([]promptdomain.Prompt, int64, error) {
	query := r.db.WithContext(ctx).Model(&promptdomain.Prompt{}).Where("user_id = ?", userID)
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(filter.Status))
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		if filter.UseFullText {
			if booleanQuery, ok := buildBooleanQuery(q); ok {
				query = query.Where("MATCH(topic, tags) AGAINST (? IN BOOLEAN MODE)", booleanQuery)
			} else {
				keyword := "%" + q + "%"
				query = query.Where("(topic LIKE ? OR tags LIKE ?)", keyword, keyword)
			}
		} else {
			keyword := "%" + q + "%"
			query = query.Where("(topic LIKE ? OR tags LIKE ?)", keyword, keyword)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count prompts: %w", err)
	}

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	var records []promptdomain.Prompt
	if err := query.Order("updated_at DESC").Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("list prompts: %w", err)
	}
	return records, total, nil
}

func buildBooleanQuery(raw string) (string, bool) {
	// buildBooleanQuery 将用户输入拆分为布尔模式查询，并追加通配符以实现前缀匹配。
	// 若命中中文或非 ASCII 字符，则返回 false 交由 LIKE 兜底处理。
	tokens := strings.Fields(raw)
	if len(tokens) == 0 {
		return "", false
	}
	clauses := make([]string, 0, len(tokens))
	for _, token := range tokens {
		cleaned := strings.Trim(token, "+-><()~*\"")
		cleaned = strings.TrimSpace(cleaned)
		if cleaned == "" {
			continue
		}
		if !isASCIIAlphaNumeric(cleaned) {
			return "", false
		}
		clause := "+" + cleaned
		if len(cleaned) >= 3 {
			clause += "*"
		}
		clauses = append(clauses, clause)
	}
	if len(clauses) == 0 {
		return "", false
	}
	return strings.Join(clauses, " "), true
}

func isASCIIAlphaNumeric(s string) bool {
	// isASCIIAlphaNumeric 判断字符串是否仅包含 ASCII 字母或数字，防止布尔查询遇到中文时报错。
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
		if !(unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}

// Delete 移除指定用户的 Prompt，并级联删除关联数据。
func (r *PromptRepository) Delete(ctx context.Context, userID, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("prompt_id = ?", id).Delete(&promptdomain.PromptKeyword{})
		if res.Error != nil {
			return fmt.Errorf("delete prompt keywords: %w", res.Error)
		}
		if res := tx.Where("prompt_id = ?", id).Delete(&promptdomain.PromptVersion{}); res.Error != nil {
			return fmt.Errorf("delete prompt versions: %w", res.Error)
		}
		res = tx.Where("id = ? AND user_id = ?", id, userID).Delete(&promptdomain.Prompt{})
		if res.Error != nil {
			return fmt.Errorf("delete prompt: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
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
		stored.Weight = entity.Weight
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
