package promptcomment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	promptdomain "electron-go-app/backend/internal/domain/prompt"
	repository "electron-go-app/backend/internal/repository"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	defaultCommentPageSize    = 10
	defaultCommentMaxPageSize = 60
)

var (
	// ErrPromptNotFound 表示目标 Prompt 不存在。
	ErrPromptNotFound = errors.New("目标 Prompt 不存在")
	// ErrParentCommentInvalid 表示回复的目标评论无效。
	ErrParentCommentInvalid = errors.New("回复目标评论不存在或不可用")
	// ErrCommentNotFound 表示评论不存在。
	ErrCommentNotFound = errors.New("评论不存在")
	// ErrCommentNotApproved 表示评论尚未通过审核，暂不允许互动。
	ErrCommentNotApproved = errors.New("评论尚未通过审核，暂无法点赞")
)

// AuditFunc 定义内容审核的函数签名，便于在测试中注入假实现。
type AuditFunc func(ctx context.Context, userID uint, content string) error

// Config 描述评论服务的可配置参数。
type Config struct {
	DefaultPageSize int
	MaxPageSize     int
	RequireApproval bool
	MaxBodyLength   int
	LikeDeltaStep   int
}

// Service 负责 Prompt 评论的业务逻辑。
type Service struct {
	comments        *repository.PromptCommentRepository
	likes           *repository.PromptCommentLikeRepository
	prompts         *repository.PromptRepository
	users           *repository.UserRepository
	logger          *zap.SugaredLogger
	defaultPageSize int
	maxPageSize     int
	requireApproval bool
	maxBodyLength   int
	auditFn         AuditFunc
	likeDeltaStep   int
}

// NewService 创建评论服务实例。
func NewService(comments *repository.PromptCommentRepository, likes *repository.PromptCommentLikeRepository, prompts *repository.PromptRepository, users *repository.UserRepository, audit AuditFunc, logger *zap.SugaredLogger, cfg Config) *Service {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	if cfg.DefaultPageSize <= 0 {
		cfg.DefaultPageSize = defaultCommentPageSize
	}
	if cfg.MaxPageSize <= 0 {
		cfg.MaxPageSize = defaultCommentMaxPageSize
	}
	if cfg.DefaultPageSize > cfg.MaxPageSize {
		cfg.DefaultPageSize = cfg.MaxPageSize
	}
	if cfg.LikeDeltaStep <= 0 {
		cfg.LikeDeltaStep = 1
	}
	return &Service{
		comments:        comments,
		likes:           likes,
		prompts:         prompts,
		users:           users,
		logger:          logger,
		defaultPageSize: cfg.DefaultPageSize,
		maxPageSize:     cfg.MaxPageSize,
		requireApproval: cfg.RequireApproval,
		maxBodyLength:   cfg.MaxBodyLength,
		auditFn:         audit,
		likeDeltaStep:   cfg.LikeDeltaStep,
	}
}

// CreateCommentInput 描述创建评论所需的字段。
type CreateCommentInput struct {
	PromptID uint
	UserID   uint
	ParentID *uint
	Body     string
}

// Create 创建一条新的评论记录，按需自动审核。
func (s *Service) Create(ctx context.Context, input CreateCommentInput) (*promptdomain.PromptComment, error) {
	body := strings.TrimSpace(input.Body)
	if body == "" {
		return nil, errors.New("评论内容不能为空")
	}
	if s.maxBodyLength > 0 && len([]rune(body)) > s.maxBodyLength {
		return nil, fmt.Errorf("评论长度不能超过 %d 个字符", s.maxBodyLength)
	}
	if input.PromptID == 0 || input.UserID == 0 {
		return nil, errors.New("缺少必要的评论上下文")
	}
	if err := s.auditContent(ctx, input.UserID, body); err != nil {
		return nil, err
	}
	if _, err := s.prompts.FindByIDGlobal(ctx, input.PromptID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPromptNotFound
		}
		return nil, fmt.Errorf("load prompt: %w", err)
	}
	var parent *promptdomain.PromptComment
	if input.ParentID != nil && *input.ParentID != 0 {
		var err error
		parent, err = s.comments.FindByID(ctx, *input.ParentID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrParentCommentInvalid
			}
			return nil, fmt.Errorf("load parent comment: %w", err)
		}
		if parent.PromptID != input.PromptID {
			return nil, ErrParentCommentInvalid
		}
		if parent.Status != promptdomain.PromptCommentStatusApproved && s.requireApproval {
			return nil, errors.New("目标评论尚未通过审核，暂无法回复")
		}
	}
	status := promptdomain.PromptCommentStatusApproved
	if s.requireApproval {
		status = promptdomain.PromptCommentStatusPending
	}
	comment := &promptdomain.PromptComment{
		PromptID: input.PromptID,
		UserID:   input.UserID,
		ParentID: input.ParentID,
		RootID:   nil,
		Body:     body,
		Status:   status,
	}
	if parent != nil {
		rootID := parent.ID
		if parent.RootID != nil && *parent.RootID != 0 {
			rootID = *parent.RootID
		}
		comment.RootID = &rootID
	}
	if err := s.comments.Create(ctx, comment); err != nil {
		return nil, err
	}
	if comment.ParentID == nil || *comment.ParentID == 0 {
		if err := s.comments.UpdateRootID(ctx, comment.ID, comment.ID); err != nil {
			s.logger.Warnw("update comment root id failed", "error", err, "comment_id", comment.ID)
		} else {
			comment.RootID = &comment.ID
		}
	}
	if err := s.attachAuthors(ctx, []*promptdomain.PromptComment{comment}); err != nil {
		s.logger.Warnw("attach comment author failed", "error", err)
	}
	return comment, nil
}

// CommentThread 描述带回复的顶层评论。
type CommentThread struct {
	Root    promptdomain.PromptComment
	Replies []promptdomain.PromptComment
}

// ListCommentsInput 描述评论列表查询参数。
type ListCommentsInput struct {
	PromptID uint
	Status   string
	Page     int
	PageSize int
	ViewerID uint
}

// ListCommentsResult 封装评论列表的返回结构。
type ListCommentsResult struct {
	Items      []CommentThread
	Page       int
	PageSize   int
	Total      int64
	TotalPages int
}

// List 返回指定 Prompt 的评论列表，包含顶层评论及其回复。
func (s *Service) List(ctx context.Context, input ListCommentsInput) (*ListCommentsResult, error) {
	if input.PromptID == 0 {
		return nil, errors.New("缺少 Prompt 编号")
	}
	page := input.Page
	if page <= 0 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = s.defaultPageSize
	}
	if s.maxPageSize > 0 && pageSize > s.maxPageSize {
		pageSize = s.maxPageSize
	}
	filter := repository.PromptCommentListFilter{
		PromptID: input.PromptID,
		Status:   strings.TrimSpace(input.Status),
		Limit:    pageSize,
		Offset:   (page - 1) * pageSize,
	}
	items, total, err := s.comments.ListRootComments(ctx, filter)
	if err != nil {
		return nil, err
	}
	rootIDs := make([]uint, 0, len(items))
	for _, item := range items {
		rootIDs = append(rootIDs, item.ID)
	}
	replies, err := s.comments.ListReplies(ctx, input.PromptID, rootIDs, filter.Status)
	if err != nil {
		return nil, err
	}
	replyCount, err := s.comments.CountRepliesByRoot(ctx, input.PromptID, rootIDs, filter.Status)
	if err != nil {
		return nil, err
	}
	commentPtrs := make([]*promptdomain.PromptComment, 0, len(items)+len(replies))
	for i := range items {
		commentPtrs = append(commentPtrs, &items[i])
	}
	for i := range replies {
		commentPtrs = append(commentPtrs, &replies[i])
	}
	if err := s.attachAuthors(ctx, commentPtrs); err != nil {
		s.logger.Warnw("attach comment authors failed", "error", err)
	}
	if err := s.attachLikes(ctx, commentPtrs, input.ViewerID); err != nil {
		s.logger.Warnw("attach comment likes failed", "error", err)
	}
	repliesByRoot := make(map[uint][]promptdomain.PromptComment, len(rootIDs))
	for _, reply := range replies {
		if reply.RootID == nil {
			continue
		}
		repliesByRoot[*reply.RootID] = append(repliesByRoot[*reply.RootID], reply)
	}
	threads := make([]CommentThread, 0, len(items))
	for _, root := range items {
		c := root
		c.ReplyCount = int(replyCountForRoot(replyCount, root.ID))
		threads = append(threads, CommentThread{
			Root:    c,
			Replies: repliesByRoot[root.ID],
		})
	}
	totalPages := 0
	if pageSize > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return &ListCommentsResult{
		Items:      threads,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

// replyCountForRoot 返回指定顶层评论的回复数量，缺失时默认为 0。
func replyCountForRoot(counts map[uint]int64, rootID uint) int64 {
	if val, ok := counts[rootID]; ok {
		return val
	}
	return 0
}

// UpdateCommentLikeInput 描述点赞或取消点赞评论时的输入参数。
type UpdateCommentLikeInput struct {
	CommentID uint
	UserID    uint
}

// UpdateCommentLikeOutput 返回点赞变更后的评论点赞数据。
type UpdateCommentLikeOutput struct {
	LikeCount uint `json:"like_count"`
	Liked     bool `json:"liked"`
}

// ReviewInput 描述评论审核所需字段。
type ReviewInput struct {
	CommentID uint
	Reviewer  uint
	Status    string
	Note      string
}

// Review 更新评论的审核状态。
func (s *Service) Review(ctx context.Context, input ReviewInput) (*promptdomain.PromptComment, error) {
	status := strings.TrimSpace(input.Status)
	switch status {
	case promptdomain.PromptCommentStatusPending, promptdomain.PromptCommentStatusApproved, promptdomain.PromptCommentStatusRejected:
		// 合法状态
	default:
		return nil, errors.New("不支持的评论状态")
	}
	entity, err := s.comments.FindByID(ctx, input.CommentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCommentNotFound
		}
		return nil, err
	}
	if err := s.comments.UpdateStatus(ctx, input.CommentID, status, &input.Reviewer, strings.TrimSpace(input.Note)); err != nil {
		return nil, err
	}
	entity.Status = status
	entity.ReviewerUserID = &input.Reviewer
	entity.ReviewNote = strings.TrimSpace(input.Note)
	if err := s.attachAuthors(ctx, []*promptdomain.PromptComment{entity}); err != nil {
		s.logger.Warnw("attach reviewer author failed", "error", err)
	}
	return entity, nil
}

// Delete 删除指定评论，若为楼主则级联删除其全部回复。
func (s *Service) Delete(ctx context.Context, commentID uint) error {
	if commentID == 0 {
		return errors.New("评论编号不能为空")
	}
	entity, err := s.comments.FindByID(ctx, commentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCommentNotFound
		}
		return err
	}
	if err := s.comments.DeleteCascade(ctx, entity.ID); err != nil {
		return err
	}
	return nil
}

// LikeComment 处理用户点赞评论的请求。
func (s *Service) LikeComment(ctx context.Context, input UpdateCommentLikeInput) (UpdateCommentLikeOutput, error) {
	return s.changeCommentLike(ctx, input, true)
}

// UnlikeComment 处理用户取消点赞评论的请求。
func (s *Service) UnlikeComment(ctx context.Context, input UpdateCommentLikeInput) (UpdateCommentLikeOutput, error) {
	return s.changeCommentLike(ctx, input, false)
}

// changeCommentLike 根据动作新增或移除评论点赞关系。
func (s *Service) changeCommentLike(ctx context.Context, input UpdateCommentLikeInput, like bool) (UpdateCommentLikeOutput, error) {
	if input.CommentID == 0 || input.UserID == 0 {
		return UpdateCommentLikeOutput{}, errors.New("评论编号和用户编号不能为空")
	}
	if s.likes == nil {
		return UpdateCommentLikeOutput{}, errors.New("评论点赞仓储未初始化")
	}
	entity, err := s.comments.FindByID(ctx, input.CommentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return UpdateCommentLikeOutput{}, ErrCommentNotFound
		}
		return UpdateCommentLikeOutput{}, err
	}
	if entity.Status != promptdomain.PromptCommentStatusApproved {
		return UpdateCommentLikeOutput{}, ErrCommentNotApproved
	}
	var delta int
	step := s.likeDeltaStep
	if like {
		created, err := s.likes.AddLike(ctx, input.CommentID, input.UserID)
		if err != nil {
			return UpdateCommentLikeOutput{}, fmt.Errorf("add comment like: %w", err)
		}
		if created {
			delta = step
		}
	} else {
		removed, err := s.likes.RemoveLike(ctx, input.CommentID, input.UserID)
		if err != nil {
			return UpdateCommentLikeOutput{}, fmt.Errorf("remove comment like: %w", err)
		}
		if removed {
			delta = -step
		}
	}
	if delta != 0 {
		if err := s.comments.IncrementLikeCount(ctx, input.CommentID, delta); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return UpdateCommentLikeOutput{}, ErrCommentNotFound
			}
			return UpdateCommentLikeOutput{}, fmt.Errorf("update comment like count: %w", err)
		}
	}
	refreshed, err := s.comments.FindByID(ctx, input.CommentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return UpdateCommentLikeOutput{}, ErrCommentNotFound
		}
		return UpdateCommentLikeOutput{}, err
	}
	if err := s.attachLikes(ctx, []*promptdomain.PromptComment{refreshed}, input.UserID); err != nil {
		s.logger.Warnw("attach like for comment failed", "error", err, "comment_id", input.CommentID, "user_id", input.UserID)
	}
	return UpdateCommentLikeOutput{
		LikeCount: refreshed.LikeCount,
		Liked:     refreshed.IsLiked,
	}, nil
}

// attachAuthors 批量补齐评论作者信息，提升界面展示的完整度。
func (s *Service) attachAuthors(ctx context.Context, comments []*promptdomain.PromptComment) error {
	ids := make([]uint, 0, len(comments))
	for _, comment := range comments {
		if comment == nil || comment.UserID == 0 {
			continue
		}
		ids = append(ids, comment.UserID)
	}
	if len(ids) == 0 {
		return nil
	}
	userMap, err := s.users.ListByIDs(ctx, uniqueUint(ids))
	if err != nil {
		return err
	}
	for _, comment := range comments {
		if comment == nil {
			continue
		}
		if user := userMap[comment.UserID]; user != nil {
			comment.Author = &promptdomain.UserBrief{
				ID:        user.ID,
				Username:  user.Username,
				Email:     user.Email,
				AvatarURL: user.AvatarURL,
				Headline:  user.ProfileHeadline,
				Bio:       user.ProfileBio,
				Location:  user.ProfileLocation,
				Website:   user.ProfileWebsite,
				BannerURL: user.ProfileBannerURL,
			}
		}
	}
	return nil
}

// attachLikes 根据用户上下文补齐评论的点赞态。
func (s *Service) attachLikes(ctx context.Context, comments []*promptdomain.PromptComment, viewerID uint) error {
	if len(comments) == 0 {
		return nil
	}
	for _, comment := range comments {
		if comment == nil {
			continue
		}
		comment.IsLiked = false
	}
	if viewerID == 0 || s.likes == nil {
		return nil
	}
	ids := make([]uint, 0, len(comments))
	for _, comment := range comments {
		if comment == nil || comment.ID == 0 {
			continue
		}
		ids = append(ids, comment.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	likedMap, err := s.likes.ListUserLikedCommentIDs(ctx, viewerID, uniqueUint(ids))
	if err != nil {
		return err
	}
	for _, comment := range comments {
		if comment == nil {
			continue
		}
		comment.IsLiked = likedMap[comment.ID]
	}
	return nil
}

// uniqueUint 对用户编号去重，避免重复查库。
func uniqueUint(values []uint) []uint {
	m := make(map[uint]struct{}, len(values))
	for _, v := range values {
		m[v] = struct{}{}
	}
	result := make([]uint, 0, len(m))
	for v := range m {
		result = append(result, v)
	}
	return result
}

// auditContent 对评论正文执行内容审核。
func (s *Service) auditContent(ctx context.Context, userID uint, body string) error {
	if s.auditFn == nil {
		return nil
	}
	return s.auditFn(ctx, userID, body)
}
