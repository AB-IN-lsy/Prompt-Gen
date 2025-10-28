package publicprompt

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	promptdomain "electron-go-app/backend/internal/domain/prompt"
	"electron-go-app/backend/internal/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
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

// visitIncrement 定义每次访问需要累加的基准值。
const visitIncrement = 1

// normaliseVisitConfig 负责填充访问量配置的默认值。
func normaliseVisitConfig(cfg VisitConfig) VisitConfig {
	if cfg.BufferKey == "" {
		cfg.BufferKey = "prompt:visit:buffer"
	}
	if cfg.GuardPrefix == "" {
		cfg.GuardPrefix = "prompt:visit:guard"
	}
	if cfg.FlushLockKey == "" {
		cfg.FlushLockKey = "prompt:visit:flush:lock"
	}
	if cfg.GuardTTL <= 0 {
		cfg.GuardTTL = time.Minute
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = time.Minute
	}
	if cfg.FlushBatch <= 0 {
		cfg.FlushBatch = 128
	}
	if cfg.FlushLockTTL <= 0 {
		cfg.FlushLockTTL = 10 * time.Second
	}
	return cfg
}

const (
	defaultScoreLikeWeight        = 3.0
	defaultScoreDownloadWeight    = 4.0
	defaultScoreVisitWeight       = 1.0
	defaultScoreRecencyWeight     = 2.0
	defaultScoreBase              = 1.0
	defaultScoreHalfLife          = 24 * time.Hour
	defaultScoreRefreshInterval   = 5 * time.Minute
	defaultScoreRefreshBatchLimit = 200
)

// normaliseScoreConfig 负责为评分配置补全默认值，避免缺失参数导致评分流程失效。
func normaliseScoreConfig(cfg ScoreConfig) ScoreConfig {
	if cfg.LikeWeight <= 0 {
		cfg.LikeWeight = defaultScoreLikeWeight
	}
	if cfg.DownloadWeight <= 0 {
		cfg.DownloadWeight = defaultScoreDownloadWeight
	}
	if cfg.VisitWeight <= 0 {
		cfg.VisitWeight = defaultScoreVisitWeight
	}
	if cfg.RecencyWeight < 0 {
		cfg.RecencyWeight = defaultScoreRecencyWeight
	}
	if cfg.BaseScore < 0 {
		cfg.BaseScore = defaultScoreBase
	}
	if cfg.RecencyHalfLife <= 0 {
		cfg.RecencyHalfLife = defaultScoreHalfLife
	}
	if cfg.RefreshInterval <= 0 {
		cfg.RefreshInterval = defaultScoreRefreshInterval
	}
	if cfg.RefreshBatch <= 0 {
		cfg.RefreshBatch = defaultScoreRefreshBatchLimit
	}
	return cfg
}

// VisitConfig 描述访问量统计所需的配置项。
type VisitConfig struct {
	Enabled       bool
	BufferKey     string
	GuardPrefix   string
	GuardTTL      time.Duration
	FlushInterval time.Duration
	FlushBatch    int
	FlushLockKey  string
	FlushLockTTL  time.Duration
}

// ScoreConfig 描述公共 Prompt 质量评分的权重配置。
type ScoreConfig struct {
	Enabled         bool
	LikeWeight      float64
	DownloadWeight  float64
	VisitWeight     float64
	RecencyWeight   float64
	RecencyHalfLife time.Duration
	BaseScore       float64
	RefreshInterval time.Duration
	RefreshBatch    int
}

// Config 描述公共库服务的可配置参数。
type Config struct {
	DefaultPageSize int
	MaxPageSize     int
	Visit           VisitConfig
	Score           ScoreConfig
}

// Service 封装公共 Prompt 库相关的业务逻辑。
type Service struct {
	repo            *repository.PublicPromptRepository
	db              *gorm.DB
	prompts         *repository.PromptRepository
	logger          *zap.SugaredLogger
	allowSubmission bool
	metricsEnabled  bool
	redis           *redis.Client
	visitCfg        VisitConfig
	visitLockValue  string
	defaultPageSize int
	maxPageSize     int
	scoreEnabled    bool
	scoreCfg        ScoreConfig
}

// NewService 创建公共 Prompt 服务。
func NewService(repo *repository.PublicPromptRepository, db *gorm.DB, logger *zap.SugaredLogger, allowSubmission bool) *Service {
	return NewServiceWithConfig(repo, db, logger, allowSubmission, Config{}, nil)
}

// NewServiceWithConfig 创建公共 Prompt 服务，允许自定义分页配置。
func NewServiceWithConfig(repo *repository.PublicPromptRepository, db *gorm.DB, logger *zap.SugaredLogger, allowSubmission bool, cfg Config, redisClient *redis.Client) *Service {
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
	cfg.Visit = normaliseVisitConfig(cfg.Visit)
	cfg.Score = normaliseScoreConfig(cfg.Score)
	// 初始化访问量统计和评分系统的启用状态。只有在“在线模式 + 访问统计开关打开 + 配好了 Redis”时 才启用访问量统计。
	metricsEnabled := allowSubmission && cfg.Visit.Enabled && redisClient != nil
	scoreEnabled := cfg.Score.Enabled
	return &Service{
		repo:            repo,
		db:              db,
		prompts:         repository.NewPromptRepository(db),
		logger:          logger,
		allowSubmission: allowSubmission,
		metricsEnabled:  metricsEnabled,
		redis:           redisClient,
		visitCfg:        cfg.Visit,
		visitLockValue:  uuid.NewString(),
		defaultPageSize: cfg.DefaultPageSize,
		maxPageSize:     cfg.MaxPageSize,
		scoreEnabled:    scoreEnabled,
		scoreCfg:        cfg.Score,
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
	SortBy       string
	SortOrder    string
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
		SortBy:       filter.SortBy,
		SortOrder:    filter.SortOrder,
	}
	items, total, err := s.repo.List(ctx, repoFilter)
	if err != nil {
		return nil, err
	}
	if err := s.fillLikeSnapshot(ctx, filter.ViewerUserID, items); err != nil {
		return nil, err
	}
	if err := s.fillVisitSnapshot(ctx, items); err != nil {
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
	// 填充点赞数据与访问量数据。
	if err := s.populateLikeSnapshot(ctx, viewerUserID, entity); err != nil {
		return nil, err
	}
	if err := s.populateVisitSnapshot(ctx, entity); err != nil {
		return nil, err
	}
	s.trackVisit(ctx, entity, viewerUserID)
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

// populateVisitSnapshot 用于读取公共 Prompt 关联的源 Prompt 访问次数并写回，缺少源 Prompt 时默认返回 0。
func (s *Service) populateVisitSnapshot(ctx context.Context, entity *promptdomain.PublicPrompt) error {
	if entity == nil || entity.SourcePromptID == nil || *entity.SourcePromptID == 0 {
		entity.VisitCount = 0
		return nil
	}
	prompt, err := s.prompts.FindByIDGlobal(ctx, *entity.SourcePromptID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			entity.VisitCount = 0
			return nil
		}
		return fmt.Errorf("load prompt visit count: %w", err)
	}
	entity.VisitCount = prompt.VisitCount
	entity.VisitCount += s.pendingVisitDelta(ctx, *entity.SourcePromptID)
	return nil
}

// fillVisitSnapshot 将访问次数批量写入公共库条目，便于列表和卡片直接展示。
func (s *Service) fillVisitSnapshot(ctx context.Context, items []promptdomain.PublicPrompt) error {
	for i := range items {
		if err := s.populateVisitSnapshot(ctx, &items[i]); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
	}
	return nil
}

// pendingVisitDelta 负责读取 Redis 缓冲区中的访问增量，便于展示与评分时叠加尚未落库的数据。
func (s *Service) pendingVisitDelta(ctx context.Context, promptID uint) uint64 {
	if !s.metricsEnabled || s.redis == nil || promptID == 0 {
		return 0
	}
	field := s.visitFieldKey(promptID)
	pending, err := s.redis.HGet(ctx, s.visitCfg.BufferKey, field).Result()
	if err != nil {
		if err != redis.Nil {
			s.logger.Warnw("load pending visit count failed", "error", err, "prompt_id", promptID)
		}
		return 0
	}
	delta, convErr := strconv.ParseInt(pending, 10, 64)
	if convErr != nil {
		s.logger.Warnw("parse pending visit count failed", "error", convErr, "prompt_id", promptID, "raw", pending)
		return 0
	}
	if delta <= 0 {
		return 0
	}
	return uint64(delta)
}

// calculateQualityScore 按权重组合点赞、下载、访问与时效性因素，输出最终评分。
func (s *Service) calculateQualityScore(downloads uint, likes uint, visits uint64, updatedAt time.Time) float64 {
	if !s.scoreEnabled {
		return 0
	}
	cfg := s.scoreCfg
	score := cfg.BaseScore
	score += cfg.DownloadWeight * math.Log1p(float64(downloads))
	score += cfg.LikeWeight * math.Log1p(float64(likes))
	score += cfg.VisitWeight * math.Log1p(float64(visits))
	if cfg.RecencyWeight > 0 && !updatedAt.IsZero() {
		halfLife := cfg.RecencyHalfLife
		if halfLife <= 0 {
			halfLife = defaultScoreHalfLife
		}
		age := time.Since(updatedAt)
		if age < 0 {
			age = 0
		}
		if halfLife > 0 {
			decay := math.Exp(-age.Hours() / halfLife.Hours())
			score += cfg.RecencyWeight * decay
		}
	}
	return score
}

// trackVisit 在详情接口被访问时自增访问次数，仅在在线模式启用，并结合 Redis 做去重缓冲。
func (s *Service) trackVisit(ctx context.Context, entity *promptdomain.PublicPrompt, viewerUserID uint) {
	if !s.metricsEnabled || entity == nil || entity.SourcePromptID == nil || *entity.SourcePromptID == 0 {
		return
	}
	promptID := *entity.SourcePromptID
	if viewerUserID != 0 && !s.acquireVisitGuard(ctx, promptID, viewerUserID) {
		return
	}
	if s.enqueueVisit(ctx, promptID, visitIncrement) {
		entity.VisitCount += uint64(visitIncrement)
	}
}

// visitFieldKey 生成访问量缓冲区使用的字段键。
func (s *Service) visitFieldKey(promptID uint) string {
	return strconv.FormatUint(uint64(promptID), 10)
}

// acquireVisitGuard 基于用户维度实现短期去重，防止刷新请求刷爆访问量。
func (s *Service) acquireVisitGuard(ctx context.Context, promptID, userID uint) bool {
	if s.redis == nil {
		return true
	}
	guardKey := fmt.Sprintf("%s:%d:%d", s.visitCfg.GuardPrefix, promptID, userID)
	ok, err := s.redis.SetNX(ctx, guardKey, "1", s.visitCfg.GuardTTL).Result()
	if err != nil {
		s.logger.Warnw("acquire visit guard failed", "error", err, "prompt_id", promptID, "user_id", userID)
		return true
	}
	return ok
}

// enqueueVisit 将访问量写入 Redis 缓冲区，写入失败时回退到同步更新数据库。
func (s *Service) enqueueVisit(ctx context.Context, promptID uint, delta int) bool {
	if delta == 0 {
		return true
	}
	if s.redis == nil {
		if err := s.prompts.IncrementVisitCount(ctx, promptID, delta); err != nil {
			s.logger.Warnw("increment prompt visit failed", "error", err, "prompt_id", promptID)
			return false
		}
		return true
	}
	field := s.visitFieldKey(promptID)
	if err := s.redis.HIncrBy(ctx, s.visitCfg.BufferKey, field, int64(delta)).Err(); err != nil {
		s.logger.Warnw("buffer prompt visit failed", "error", err, "prompt_id", promptID)
		if err := s.prompts.IncrementVisitCount(ctx, promptID, delta); err != nil {
			s.logger.Errorw("fallback increment prompt visit failed", "error", err, "prompt_id", promptID)
			return false
		}
	}
	return true
}

// recomputeAndPersistQualityScore 基于当前点赞、下载与访问数据重新计算并落库质量评分。
func (s *Service) recomputeAndPersistQualityScore(ctx context.Context, entity *promptdomain.PublicPrompt) {
	if !s.scoreEnabled || entity == nil {
		return
	}
	var (
		likeCount uint
		visitSum  uint64
	)
	if entity.SourcePromptID != nil && *entity.SourcePromptID != 0 {
		prompt, err := s.prompts.FindByIDGlobal(ctx, *entity.SourcePromptID)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				s.logger.Warnw("load source prompt metrics failed", "error", err, "public_prompt_id", entity.ID, "prompt_id", *entity.SourcePromptID)
			}
		} else {
			likeCount = prompt.LikeCount
			visitSum = prompt.VisitCount + s.pendingVisitDelta(ctx, *entity.SourcePromptID)
		}
	}
	score := s.calculateQualityScore(entity.DownloadCount, likeCount, visitSum, entity.UpdatedAt)
	if err := s.repo.UpdateQualityScore(ctx, entity.ID, score); err != nil {
		s.logger.Warnw("update public prompt quality score failed", "error", err, "public_prompt_id", entity.ID)
		return
	}
	entity.QualityScore = score
}

// refreshAllQualityScores 批量重算公共 Prompt 的质量评分，按主键游标分页处理。
func (s *Service) refreshAllQualityScores(ctx context.Context) error {
	if !s.scoreEnabled {
		return nil
	}
	batchSize := s.scoreCfg.RefreshBatch
	if batchSize <= 0 {
		batchSize = defaultScoreRefreshBatchLimit
	}
	var afterID uint
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		items, err := s.repo.ListForScore(ctx, afterID, batchSize)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		for i := range items {
			s.recomputeAndPersistQualityScore(ctx, &items[i])
		}
		afterID = items[len(items)-1].ID
		if len(items) < batchSize {
			return nil
		}
	}
}

// StartVisitFlushWorker 启动访问量落库任务，将 Redis 缓冲数据批量刷回 MySQL。
func (s *Service) StartVisitFlushWorker(ctx context.Context) {
	if !s.metricsEnabled || s.redis == nil {
		s.logger.Infow("visit flush worker disabled")
		return
	}
	interval := s.visitCfg.FlushInterval
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.flushVisitBuffer(ctx)
			}
		}
	}()
}

// StartScoreRefreshWorker 启动定时评分刷新任务，按配置周期重算质量评分。
// 定时任务每次执行 refreshAllQualityScores，会在这一轮里按批次遍历所有 public_prompts，把评分全部重算更新；“批量”指的是它按分页一批批处理，而不是一次 SQL 改全表。
func (s *Service) StartScoreRefreshWorker(ctx context.Context) {
	if !s.scoreEnabled || s.scoreCfg.RefreshInterval <= 0 {
		s.logger.Infow("quality score worker disabled")
		return
	}
	interval := s.scoreCfg.RefreshInterval
	if err := s.refreshAllQualityScores(ctx); err != nil && !errors.Is(err, context.Canceled) {
		s.logger.Warnw("initial quality score refresh failed", "error", err)
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				workCtx, cancel := context.WithTimeout(ctx, interval)
				if err := s.refreshAllQualityScores(workCtx); err != nil && !errors.Is(err, context.Canceled) {
					s.logger.Warnw("refresh public prompt quality score failed", "error", err)
				}
				cancel()
			}
		}
	}()
}

// flushVisitBuffer 是访问量落库的后台任务，专门把 Redis 里缓冲的浏览次数刷回 MySQL，避免实时写库造成压力。它的作用流程是：
/*
  1. 取分布式锁：先通过 SetNX 拿到 PUBLIC_PROMPT_VISIT_FLUSH_LOCK_KEY 锁，确保多实例不会同时刷同一个缓冲。
  2. 按批扫描 Redis Hash：对配置的缓冲键（默认 prompt:visit:buffer）做 HSCAN，每次取一批键值对。键是源 Prompt ID，值是累积的访问增量。
  3. 写入 MySQL：把每个键值（Prompt ID + 增量）传给 PromptRepository.IncrementVisitCount，让 prompts.visit_count 累加相应次数。
  4. 清理缓冲：对成功写入的字段执行 HDEL，保证同一批数据不会重复刷。
*/
func (s *Service) flushVisitBuffer(ctx context.Context) {
	if !s.metricsEnabled || s.redis == nil {
		return
	}
	lockCtx, cancel := context.WithTimeout(ctx, s.visitCfg.FlushLockTTL)
	defer cancel()
	if !s.acquireVisitLock(lockCtx) {
		return
	}
	defer s.releaseVisitLock(context.Background())

	cursor := uint64(0)
	processed := 0
	limit := s.visitCfg.FlushBatch
	if limit <= 0 {
		limit = 128
	}
	for {
		if processed >= limit {
			return
		}
		scanCount := int64(limit - processed)
		if scanCount <= 0 {
			scanCount = int64(limit)
		}
		results, nextCursor, err := s.redis.HScan(ctx, s.visitCfg.BufferKey, cursor, "*", scanCount).Result()
		if err != nil {
			if !errors.Is(err, redis.Nil) {
				s.logger.Warnw("scan visit buffer failed", "error", err)
			}
			return
		}
		if len(results) == 0 {
			if nextCursor == 0 {
				return
			}
			cursor = nextCursor
			continue
		}
		type visitEntry struct {
			promptID uint
			delta    int
			field    string
		}
		entries := make([]visitEntry, 0, len(results)/2)
		for i := 0; i+1 < len(results) && processed < limit; i += 2 {
			field := results[i]
			rawDelta := results[i+1]
			promptIDVal, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				s.logger.Warnw("parse visit buffer key failed", "error", err, "field", field)
				continue
			}
			deltaVal, err := strconv.ParseInt(rawDelta, 10, 64)
			if err != nil {
				s.logger.Warnw("parse visit buffer value failed", "error", err, "field", field, "value", rawDelta)
				continue
			}
			if deltaVal <= 0 {
				continue
			}
			if deltaVal > int64(^uint(0)>>1) {
				deltaVal = int64(^uint(0) >> 1)
			}
			entries = append(entries, visitEntry{
				promptID: uint(promptIDVal),
				delta:    int(deltaVal),
				field:    field,
			})
			processed++
		}
		if len(entries) == 0 {
			if nextCursor == 0 {
				return
			}
			cursor = nextCursor
			continue
		}
		removedFields := make([]string, 0, len(entries))
		for _, item := range entries {
			if err := s.prompts.IncrementVisitCount(ctx, item.promptID, item.delta); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					removedFields = append(removedFields, item.field)
					continue
				}
				s.logger.Warnw("flush visit count failed", "error", err, "prompt_id", item.promptID, "delta", item.delta)
				continue
			}
			removedFields = append(removedFields, item.field)
		}
		if len(removedFields) > 0 {
			if err := s.redis.HDel(ctx, s.visitCfg.BufferKey, removedFields...).Err(); err != nil {
				s.logger.Warnw("cleanup visit buffer failed", "error", err, "fields", removedFields)
			}
		}
		if nextCursor == 0 {
			return
		}
		cursor = nextCursor
	}
}

// acquireVisitLock 获取访问量刷库的分布式锁，避免多实例重复刷写。
func (s *Service) acquireVisitLock(ctx context.Context) bool {
	if s.redis == nil {
		return false
	}
	ok, err := s.redis.SetNX(ctx, s.visitCfg.FlushLockKey, s.visitLockValue, s.visitCfg.FlushLockTTL).Result()
	if err != nil {
		s.logger.Warnw("acquire visit flush lock failed", "error", err)
		return false
	}
	return ok
}

// releaseVisitLock 释放访问量刷库锁，确保只删除自己持有的锁。
func (s *Service) releaseVisitLock(ctx context.Context) {
	if s.redis == nil {
		return
	}
	const script = `
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		return redis.call("DEL", KEYS[1])
	end
	return 0
	`
	if _, err := s.redis.Eval(ctx, script, []string{s.visitCfg.FlushLockKey}, s.visitLockValue).Result(); err != nil && err != redis.Nil {
		s.logger.Warnw("release visit flush lock failed", "error", err)
	}
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
