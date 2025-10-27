package publicprompt

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	promptdomain "electron-go-app/backend/internal/domain/prompt"
	"electron-go-app/backend/internal/repository"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ErrSubmissionDisabled 表示当前运行模式不允许提交公共 Prompt。
var ErrSubmissionDisabled = errors.New("公共 Prompt 提交功能已关闭")

// ErrPublicPromptNotFound 表示指定的公共 Prompt 不存在。
var ErrPublicPromptNotFound = errors.New("公共 Prompt 不存在")

// ErrReviewStatusInvalid 表示审核状态不合法。
var ErrReviewStatusInvalid = errors.New("审核状态不合法")

// ErrPromptNotApproved 表示公共 Prompt 尚未通过审核，无法下载。
var ErrPromptNotApproved = errors.New("公共 Prompt 尚未通过审核")

// ErrLikeNotAvailable 表示当前公共 Prompt 无法执行点赞操作（缺少源 Prompt）。
var ErrLikeNotAvailable = errors.New("当前公共 Prompt 暂不支持点赞")

// DefaultListPageSize 定义公共库列表默认每页条目数。
const DefaultListPageSize = 9

// DefaultListMaxPageSize 定义公共库列表允许的最大单页条目数。
const DefaultListMaxPageSize = 60

// Config 描述公共库服务的可配置参数。
type Config struct {
	DefaultPageSize int
	MaxPageSize     int
}

// Service 封装公共 Prompt 库相关的业务逻辑。
type Service struct {
	repo            *repository.PublicPromptRepository
	db              *gorm.DB
	prompts         *repository.PromptRepository
	logger          *zap.SugaredLogger
	allowSubmission bool
	defaultPageSize int
	maxPageSize     int
}

// NewService 创建公共 Prompt 服务。
func NewService(repo *repository.PublicPromptRepository, db *gorm.DB, logger *zap.SugaredLogger, allowSubmission bool) *Service {
	return NewServiceWithConfig(repo, db, logger, allowSubmission, Config{})
}

// NewServiceWithConfig 创建公共 Prompt 服务，允许自定义分页配置。
func NewServiceWithConfig(repo *repository.PublicPromptRepository, db *gorm.DB, logger *zap.SugaredLogger, allowSubmission bool, cfg Config) *Service {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	if cfg.DefaultPageSize <= 0 {
		cfg.DefaultPageSize = DefaultListPageSize
	}
	if cfg.MaxPageSize <= 0 {
		cfg.MaxPageSize = DefaultListMaxPageSize
	}
	if cfg.DefaultPageSize > cfg.MaxPageSize {
		cfg.DefaultPageSize = cfg.MaxPageSize
	}
	return &Service{
		repo:            repo,
		db:              db,
		prompts:         repository.NewPromptRepository(db),
		logger:          logger,
		allowSubmission: allowSubmission,
		defaultPageSize: cfg.DefaultPageSize,
		maxPageSize:     cfg.MaxPageSize,
	}
}

// ListFilter 描述列表查询的过滤条件。
type ListFilter struct {
	Query        string
	Status       string
	AuthorUserID uint
	OnlyApproved bool
	Page         int
	PageSize     int
	ViewerUserID uint
}

// ListResult 描述公共库列表查询的返回值。
type ListResult struct {
	Items      []promptdomain.PublicPrompt
	Page       int
	PageSize   int
	Total      int64
	TotalPages int
}

// LikeResult 描述公共 Prompt 点赞接口的返回值。
type LikeResult struct {
	LikeCount uint
	Liked     bool
}

// List 返回公共库列表数据。
func (s *Service) List(ctx context.Context, filter ListFilter) (*ListResult, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = s.defaultPageSize
	}
	if s.maxPageSize > 0 && filter.PageSize > s.maxPageSize {
		filter.PageSize = s.maxPageSize
	}
	repoFilter := repository.PublicPromptListFilter{
		Query:        filter.Query,
		Status:       filter.Status,
		AuthorUserID: filter.AuthorUserID,
		OnlyApproved: filter.OnlyApproved,
		Limit:        filter.PageSize,
		Offset:       (filter.Page - 1) * filter.PageSize,
	}
	items, total, err := s.repo.List(ctx, repoFilter)
	if err != nil {
		return nil, err
	}
	if err := s.fillLikeSnapshot(ctx, filter.ViewerUserID, items); err != nil {
		return nil, err
	}
	totalPages := 0
	if filter.PageSize > 0 {
		totalPages = int((total + int64(filter.PageSize) - 1) / int64(filter.PageSize))
	}
	return &ListResult{
		Items:      items,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

// ListPageSizeBounds 返回公共库分页的默认与最大条目数。
func (s *Service) ListPageSizeBounds() (int, int) {
	return s.defaultPageSize, s.maxPageSize
}

// Get 查询单条公共 Prompt 详情。
func (s *Service) Get(ctx context.Context, id uint, viewerUserID uint) (*promptdomain.PublicPrompt, error) {
	entity, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPublicPromptNotFound
		}
		return nil, fmt.Errorf("query public prompt: %w", err)
	}
	if err := s.populateLikeSnapshot(ctx, viewerUserID, entity); err != nil {
		return nil, err
	}
	return entity, nil
}

// Like 为公共 Prompt 点赞，底层复用个人 Prompt 的点赞逻辑。
func (s *Service) Like(ctx context.Context, userID, publicPromptID uint) (LikeResult, error) {
	return s.toggleLike(ctx, userID, publicPromptID, true)
}

// Unlike 取消公共 Prompt 点赞。
func (s *Service) Unlike(ctx context.Context, userID, publicPromptID uint) (LikeResult, error) {
	return s.toggleLike(ctx, userID, publicPromptID, false)
}

// SubmitInput 描述公共 Prompt 提交所需的字段。
type SubmitInput struct {
	AuthorUserID     uint
	SourcePromptID   *uint
	Title            string
	Topic            string
	Summary          string
	Body             string
	Instructions     string
	PositiveKeywords string
	NegativeKeywords string
	Tags             string
	Model            string
	Language         string
}

// Submit 创建一条待审核的公共 Prompt 记录。
func (s *Service) Submit(ctx context.Context, input SubmitInput) (*promptdomain.PublicPrompt, error) {
	if !s.allowSubmission {
		return nil, ErrSubmissionDisabled
	}
	if input.AuthorUserID == 0 {
		return nil, errors.New("作者信息缺失")
	}
	if strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.Topic) == "" {
		return nil, errors.New("标题或主题不能为空")
	}
	if input.SourcePromptID != nil && *input.SourcePromptID != 0 {
		promptRepo := repository.NewPromptRepository(s.db)
		prompt, err := promptRepo.FindByID(ctx, input.AuthorUserID, *input.SourcePromptID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("仅能投稿当前账户下的 Prompt")
			}
			return nil, fmt.Errorf("查询原 Prompt 失败: %w", err)
		}
		if prompt.Status != promptdomain.PromptStatusPublished {
			return nil, errors.New("仅发布后的 Prompt 可以投稿到公共库")
		}
	}
	topic := strings.TrimSpace(input.Topic)
	lang := strings.TrimSpace(input.Language)
	if lang == "" {
		lang = "zh-CN"
	}

	if existing, err := s.repo.FindByAuthorAndTopic(ctx, input.AuthorUserID, topic); err == nil {
		if existing.Status == promptdomain.PublicPromptStatusApproved {
			return nil, errors.New("该主题已在公共库发布，可直接编辑已发布条目")
		}
		existing.Title = strings.TrimSpace(input.Title)
		existing.Topic = topic
		existing.Summary = strings.TrimSpace(input.Summary)
		existing.Body = input.Body
		existing.Instructions = input.Instructions
		existing.PositiveKeywords = input.PositiveKeywords
		existing.NegativeKeywords = input.NegativeKeywords
		existing.Tags = input.Tags
		existing.Model = strings.TrimSpace(input.Model)
		existing.Language = lang
		existing.Status = promptdomain.PublicPromptStatusPending
		existing.ReviewerUserID = nil
		existing.ReviewReason = ""
		existing.SourcePromptID = input.SourcePromptID
		existing.UpdatedAt = time.Now()
		if err := s.repo.Update(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	entity := &promptdomain.PublicPrompt{
		AuthorUserID:     input.AuthorUserID,
		Title:            strings.TrimSpace(input.Title),
		Topic:            topic,
		Summary:          strings.TrimSpace(input.Summary),
		Body:             input.Body,
		Instructions:     input.Instructions,
		PositiveKeywords: input.PositiveKeywords,
		NegativeKeywords: input.NegativeKeywords,
		Tags:             input.Tags,
		Model:            strings.TrimSpace(input.Model),
		Language:         lang,
		Status:           promptdomain.PublicPromptStatusPending,
	}
	if input.SourcePromptID != nil && *input.SourcePromptID != 0 {
		entity.SourcePromptID = input.SourcePromptID
	}
	if err := s.repo.Create(ctx, entity); err != nil {
		return nil, err
	}
	return entity, nil
}

// populateLikeSnapshot 填充单条公共 Prompt 的点赞数量与当前用户点赞态度。
func (s *Service) populateLikeSnapshot(ctx context.Context, viewerUserID uint, entity *promptdomain.PublicPrompt) error {
	if entity == nil || entity.SourcePromptID == nil || *entity.SourcePromptID == 0 {
		return nil
	}
	count, liked, err := s.prompts.LikeSnapshot(ctx, *entity.SourcePromptID, viewerUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			entity.LikeCount = 0
			entity.IsLiked = false
			return nil
		}
		return err
	}
	entity.LikeCount = count
	entity.IsLiked = liked
	return nil
}

// fillLikeSnapshot 批量填充公共 Prompt 列表的点赞数据。
func (s *Service) fillLikeSnapshot(ctx context.Context, viewerUserID uint, items []promptdomain.PublicPrompt) error {
	for i := range items {
		if err := s.populateLikeSnapshot(ctx, viewerUserID, &items[i]); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
	}
	return nil
}

// toggleLike 根据操作类型为公共 Prompt 调整点赞状态并返回最新统计结果。
func (s *Service) toggleLike(ctx context.Context, userID, publicPromptID uint, like bool) (LikeResult, error) {
	if userID == 0 || publicPromptID == 0 {
		return LikeResult{}, errors.New("用户或公共 Prompt 信息缺失")
	}
	entity, err := s.repo.FindByID(ctx, publicPromptID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return LikeResult{}, ErrPublicPromptNotFound
		}
		return LikeResult{}, fmt.Errorf("query public prompt: %w", err)
	}
	if entity.SourcePromptID == nil || *entity.SourcePromptID == 0 {
		return LikeResult{}, ErrLikeNotAvailable
	}

	promptID := *entity.SourcePromptID
	var delta int
	if like {
		created, err := s.prompts.AddLike(ctx, promptID, userID)
		if err != nil {
			return LikeResult{}, fmt.Errorf("add prompt like: %w", err)
		}
		if created {
			delta = 1
		}
	} else {
		removed, err := s.prompts.RemoveLike(ctx, promptID, userID)
		if err != nil {
			return LikeResult{}, fmt.Errorf("remove prompt like: %w", err)
		}
		if removed {
			delta = -1
		}
	}

	if delta != 0 {
		if err := s.prompts.IncrementLikeCount(ctx, promptID, delta); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return LikeResult{}, ErrPublicPromptNotFound
			}
			return LikeResult{}, fmt.Errorf("update prompt like count: %w", err)
		}
	}

	count, liked, err := s.prompts.LikeSnapshot(ctx, promptID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return LikeResult{}, ErrPublicPromptNotFound
		}
		return LikeResult{}, fmt.Errorf("load prompt like snapshot: %w", err)
	}
	entity.LikeCount = count
	entity.IsLiked = liked
	return LikeResult{
		LikeCount: count,
		Liked:     liked,
	}, nil
}

// ReviewInput 描述审核公共 Prompt 所需的参数。
type ReviewInput struct {
	ReviewerUserID uint
	PromptID       uint
	Status         string
	Reason         string
}

// Review 更新公共 Prompt 的审核状态。
func (s *Service) Review(ctx context.Context, input ReviewInput) error {
	if input.ReviewerUserID == 0 {
		return errors.New("缺少审核人信息")
	}
	if input.PromptID == 0 {
		return errors.New("缺少公共 Prompt 编号")
	}
	nextStatus := strings.TrimSpace(input.Status)
	switch nextStatus {
	case promptdomain.PublicPromptStatusApproved, promptdomain.PublicPromptStatusRejected:
		// 合法状态无需处理。
	default:
		return ErrReviewStatusInvalid
	}
	entity, err := s.repo.FindByID(ctx, input.PromptID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPublicPromptNotFound
		}
		return fmt.Errorf("query public prompt: %w", err)
	}
	entity.Status = nextStatus
	entity.ReviewerUserID = &input.ReviewerUserID
	entity.ReviewReason = strings.TrimSpace(input.Reason)
	entity.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	return nil
}

// DownloadInput 描述下载公共 Prompt 所需的参数。
type DownloadInput struct {
	UserID            uint
	PublicPromptID    uint
	ForceIncludeDraft bool
}

// Download 将公共 Prompt 复制到用户私有库。
func (s *Service) Download(ctx context.Context, input DownloadInput) (*promptdomain.Prompt, error) {
	if input.UserID == 0 || input.PublicPromptID == 0 {
		return nil, errors.New("下载参数不完整")
	}

	entity, err := s.repo.FindByID(ctx, input.PublicPromptID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPublicPromptNotFound
		}
		return nil, fmt.Errorf("query public prompt: %w", err)
	}
	if entity.Status != promptdomain.PublicPromptStatusApproved && !input.ForceIncludeDraft {
		return nil, ErrPromptNotApproved
	}

	var result promptdomain.Prompt
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txPublicRepo := s.repo.WithDB(tx)
		txPromptRepo := repository.NewPromptRepository(tx)

		now := time.Now()
		newPrompt := &promptdomain.Prompt{
			UserID:           input.UserID,
			Topic:            entity.Topic,
			Body:             entity.Body,
			Instructions:     entity.Instructions,
			PositiveKeywords: entity.PositiveKeywords,
			NegativeKeywords: entity.NegativeKeywords,
			Model:            entity.Model,
			Status:           promptdomain.PromptStatusDraft,
			Tags:             entity.Tags,
			CreatedAt:        now,
			UpdatedAt:        now,
			LatestVersionNo:  1,
		}
		if err := txPromptRepo.Create(ctx, newPrompt); err != nil {
			return err
		}
		result = *newPrompt
		if err := txPublicRepo.IncrementDownload(ctx, entity.ID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// AllowSubmission 返回当前是否支持提交公共 Prompt。
func (s *Service) AllowSubmission() bool {
	return s.allowSubmission
}

// Delete 删除公共 Prompt 记录，仅限管理员流程调用。
func (s *Service) Delete(ctx context.Context, id uint) error {
	if id == 0 {
		return errors.New("缺少公共 Prompt 编号")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPublicPromptNotFound
		}
		return err
	}
	return nil
}
