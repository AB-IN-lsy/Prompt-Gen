package repository

import (
	"context"
	"fmt"

	promptdomain "electron-go-app/backend/internal/domain/prompt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PromptCommentLikeRepository 负责评论点赞关系的持久化。
type PromptCommentLikeRepository struct {
	db *gorm.DB
}

// NewPromptCommentLikeRepository 构造评论点赞仓储。
func NewPromptCommentLikeRepository(db *gorm.DB) *PromptCommentLikeRepository {
	return &PromptCommentLikeRepository{db: db}
}

// AddLike 为评论新增点赞关系，若已点赞则忽略。
func (r *PromptCommentLikeRepository) AddLike(ctx context.Context, commentID, userID uint) (bool, error) {
	like := promptdomain.PromptCommentLike{
		CommentID: commentID,
		UserID:    userID,
	}
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&like)
	if result.Error != nil {
		return false, fmt.Errorf("create prompt comment like: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

// RemoveLike 取消评论点赞关系，若记录不存在则视为无操作。
func (r *PromptCommentLikeRepository) RemoveLike(ctx context.Context, commentID, userID uint) (bool, error) {
	result := r.db.WithContext(ctx).
		Where("comment_id = ? AND user_id = ?", commentID, userID).
		Delete(&promptdomain.PromptCommentLike{})
	if result.Error != nil {
		return false, fmt.Errorf("delete prompt comment like: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

// ListUserLikedCommentIDs 返回用户已点赞的评论编号集合。
func (r *PromptCommentLikeRepository) ListUserLikedCommentIDs(ctx context.Context, userID uint, commentIDs []uint) (map[uint]bool, error) {
	result := make(map[uint]bool, len(commentIDs))
	if userID == 0 || len(commentIDs) == 0 {
		return result, nil
	}
	var likedIDs []uint
	if err := r.db.WithContext(ctx).
		Model(&promptdomain.PromptCommentLike{}).
		Where("user_id = ? AND comment_id IN ?", userID, commentIDs).
		Pluck("comment_id", &likedIDs).Error; err != nil {
		return nil, fmt.Errorf("list prompt comment likes: %w", err)
	}
	for _, id := range likedIDs {
		result[id] = true
	}
	return result, nil
}
