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

const (
	defaultListPageSize = 12
)

// Service 封装公共 Prompt 库相关的业务逻辑。
type Service struct {
	repo            *repository.PublicPromptRepository
	db              *gorm.DB
	logger          *zap.SugaredLogger
	allowSubmission bool
}

// NewService 创建公共 Prompt 服务。
func NewService(repo *repository.PublicPromptRepository, db *gorm.DB, logger *zap.SugaredLogger, allowSubmission bool) *Service {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	return &Service{
		repo:            repo,
		db:              db,
		logger:          logger,
		allowSubmission: allowSubmission,
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
}

// ListResult 描述公共库列表查询的返回值。
type ListResult struct {
	Items      []promptdomain.PublicPrompt
	Page       int
	PageSize   int
	Total      int64
	TotalPages int
}

// List 返回公共库列表数据。
func (s *Service) List(ctx context.Context, filter ListFilter) (*ListResult, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = defaultListPageSize
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

// Get 查询单条公共 Prompt 详情。
func (s *Service) Get(ctx context.Context, id uint) (*promptdomain.PublicPrompt, error) {
	entity, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPublicPromptNotFound
		}
		return nil, fmt.Errorf("query public prompt: %w", err)
	}
	return entity, nil
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
