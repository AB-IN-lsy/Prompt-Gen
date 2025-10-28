package repository

import (
	"context"
	"errors"
	"fmt"

	promptdomain "electron-go-app/backend/internal/domain/prompt"

	"gorm.io/gorm"
)

// PromptCommentListFilter 描述评论列表查询的条件。
type PromptCommentListFilter struct {
	PromptID uint
	Status   string
	Limit    int
	Offset   int
}

// PromptCommentRepository 负责评论的持久化操作。
type PromptCommentRepository struct {
	db *gorm.DB
}

// NewPromptCommentRepository 构造评论仓储。
func NewPromptCommentRepository(db *gorm.DB) *PromptCommentRepository {
	return &PromptCommentRepository{db: db}
}

// Create 写入新的评论记录。
func (r *PromptCommentRepository) Create(ctx context.Context, entity *promptdomain.PromptComment) error {
	if entity == nil {
		return errors.New("comment entity is nil")
	}
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		return fmt.Errorf("create prompt comment: %w", err)
	}
	return nil
}

// UpdateRootID 更新顶层评论编号，通常用于首评论创建后回写自身 ID。
func (r *PromptCommentRepository) UpdateRootID(ctx context.Context, commentID uint, rootID uint) error {
	result := r.db.WithContext(ctx).Model(&promptdomain.PromptComment{}).
		Where("id = ?", commentID).
		Update("root_id", rootID)
	if result.Error != nil {
		return fmt.Errorf("update comment root id: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteCascade 删除指定评论及其全部子回复。
func (r *PromptCommentRepository) DeleteCascade(ctx context.Context, commentID uint) error {
	if commentID == 0 {
		return errors.New("comment id required")
	}
	queue := []uint{commentID}
	visited := map[uint]struct{}{
		commentID: {},
	}
	for len(queue) > 0 {
		current := queue
		queue = nil
		children, err := r.listChildIDs(ctx, current)
		if err != nil {
			return err
		}
		for _, child := range children {
			if _, ok := visited[child]; ok {
				continue
			}
			visited[child] = struct{}{}
			queue = append(queue, child)
		}
	}
	ids := make([]uint, 0, len(visited))
	for id := range visited {
		ids = append(ids, id)
	}
	result := r.db.WithContext(ctx).Where("id IN ?", ids).Delete(&promptdomain.PromptComment{})
	if result.Error != nil {
		return fmt.Errorf("delete prompt comments: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// listChildIDs 返回若干父评论下直接子评论的 ID 列表。
func (r *PromptCommentRepository) listChildIDs(ctx context.Context, parentIDs []uint) ([]uint, error) {
	if len(parentIDs) == 0 {
		return []uint{}, nil
	}
	var ids []uint
	if err := r.db.WithContext(ctx).Model(&promptdomain.PromptComment{}).
		Where("parent_id IN ?", parentIDs).
		Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list comment children: %w", err)
	}
	return ids, nil
}

// FindByID 查询评论详情。
func (r *PromptCommentRepository) FindByID(ctx context.Context, id uint) (*promptdomain.PromptComment, error) {
	var entity promptdomain.PromptComment
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&entity).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

// ListRootComments 返回某个 Prompt 的顶层评论（首层楼）以及总条数，结合分页参数可逐页加载评论入口。
func (r *PromptCommentRepository) ListRootComments(ctx context.Context, filter PromptCommentListFilter) ([]promptdomain.PromptComment, int64, error) {
	if filter.PromptID == 0 {
		return nil, 0, errors.New("prompt id required")
	}
	query := r.db.WithContext(ctx).Model(&promptdomain.PromptComment{}).
		Where("prompt_id = ? AND parent_id IS NULL", filter.PromptID)
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count prompt comments: %w", err)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	query = query.Order("created_at ASC")
	var items []promptdomain.PromptComment
	if err := query.Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list prompt comments: %w", err)
	}
	return items, total, nil
}

// ListReplies 拉取一组顶层评论（通过 rootIDs 指定）下的所有子回复，可配合状态过滤重建整棵楼中楼。
func (r *PromptCommentRepository) ListReplies(ctx context.Context, promptID uint, rootIDs []uint, status string) ([]promptdomain.PromptComment, error) {
	if promptID == 0 || len(rootIDs) == 0 {
		return []promptdomain.PromptComment{}, nil
	}
	query := r.db.WithContext(ctx).Model(&promptdomain.PromptComment{}).
		Where("prompt_id = ? AND root_id IN ? AND parent_id IS NOT NULL", promptID, rootIDs)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	query = query.Order("created_at ASC")
	var replies []promptdomain.PromptComment
	if err := query.Find(&replies).Error; err != nil {
		return nil, fmt.Errorf("list comment replies: %w", err)
	}
	return replies, nil
}

// CountRepliesByRoot 统计每个顶层评论的回复数量。
func (r *PromptCommentRepository) CountRepliesByRoot(ctx context.Context, promptID uint, rootIDs []uint, status string) (map[uint]int64, error) {
	result := make(map[uint]int64, len(rootIDs))
	if promptID == 0 || len(rootIDs) == 0 {
		return result, nil
	}
	query := r.db.WithContext(ctx).Model(&promptdomain.PromptComment{}).
		Select("root_id, COUNT(*) AS total").
		Where("prompt_id = ? AND root_id IN ? AND parent_id IS NOT NULL", promptID, rootIDs).
		Group("root_id")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	type row struct {
		RootID uint
		Total  int64
	}
	var rows []row
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("count comment replies: %w", err)
	}
	for _, item := range rows {
		result[item.RootID] = item.Total
	}
	return result, nil
}

// UpdateStatus 供管理员审核接口使用，修改评论状态及审核记录（自动审核不走该方法）。
func (r *PromptCommentRepository) UpdateStatus(ctx context.Context, id uint, status string, reviewerID *uint, note string) error {
	updates := map[string]any{
		"status":      status,
		"review_note": note,
	}
	if reviewerID != nil {
		updates["reviewer_user_id"] = *reviewerID
	} else {
		updates["reviewer_user_id"] = nil
	}
	result := r.db.WithContext(ctx).Model(&promptdomain.PromptComment{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update comment status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
