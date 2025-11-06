package adminuser

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	appLogger "electron-go-app/backend/internal/infra/logger"
	"electron-go-app/backend/internal/repository"

	"go.uber.org/zap"
)

// Config 描述管理员用户总览服务的运行参数。
type Config struct {
	DefaultPageSize   int
	MaxPageSize       int
	RecentPromptLimit int
	OnlineThreshold   time.Duration
}

// Service 聚合用户与 Prompt 数据，支撑管理员用户总览页面。
type Service struct {
	cfg     Config
	users   *repository.UserRepository
	prompts *repository.PromptRepository
	logger  *zap.SugaredLogger
}

// ListParams 描述管理员用户总览列表的查询参数。
type ListParams struct {
	Page     int
	PageSize int
	Query    string
}

// OverviewResult 返回管理员用户总览的分页结果。
type OverviewResult struct {
	Items                  []OverviewItem `json:"items"`
	Total                  int64          `json:"total"`
	Page                   int            `json:"page"`
	PageSize               int            `json:"page_size"`
	OnlineThresholdSeconds int64          `json:"online_threshold_seconds"`
}

// OverviewItem 表示单个用户的聚合信息。
type OverviewItem struct {
	ID             uint            `json:"id"`
	Username       string          `json:"username"`
	Email          string          `json:"email"`
	AvatarURL      string          `json:"avatar_url"`
	IsAdmin        bool            `json:"is_admin"`
	LastLoginAt    *time.Time      `json:"last_login_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	IsOnline       bool            `json:"is_online"`
	PromptTotals   PromptTotals    `json:"prompt_totals"`
	LatestPromptAt *time.Time      `json:"latest_prompt_at,omitempty"`
	RecentPrompts  []PromptSummary `json:"recent_prompts"`
}

// PromptTotals 统计用户名下 Prompt 的数量分布。
type PromptTotals struct {
	Total     int64 `json:"total"`
	Draft     int64 `json:"draft"`
	Published int64 `json:"published"`
	Archived  int64 `json:"archived"`
}

// PromptSummary 描述用于展示的 Prompt 摘要信息。
type PromptSummary struct {
	ID        uint      `json:"id"`
	Topic     string    `json:"topic"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}

// NewService 构造管理员用户总览服务，确保配置项使用有效默认值。
func NewService(cfg Config, users *repository.UserRepository, prompts *repository.PromptRepository, logger *zap.SugaredLogger) *Service {
	if logger == nil {
		logger = appLogger.S().With("component", "service.adminuser")
	}
	if cfg.DefaultPageSize <= 0 {
		cfg.DefaultPageSize = 20
	}
	if cfg.MaxPageSize <= 0 {
		cfg.MaxPageSize = 100
	}
	if cfg.MaxPageSize < cfg.DefaultPageSize {
		cfg.MaxPageSize = cfg.DefaultPageSize
	}
	if cfg.RecentPromptLimit < 0 {
		cfg.RecentPromptLimit = 0
	}
	if cfg.OnlineThreshold <= 0 {
		cfg.OnlineThreshold = 15 * time.Minute
	}
	return &Service{
		cfg:     cfg,
		users:   users,
		prompts: prompts,
		logger:  logger,
	}
}

// ListOverview 汇总管理员所需的用户列表、在线状态及 Prompt 统计信息。
func (s *Service) ListOverview(ctx context.Context, params ListParams) (OverviewResult, error) {
	if s == nil || s.users == nil {
		return OverviewResult{}, errors.New("admin user service not initialised")
	}

	page := params.Page
	if page <= 0 {
		page = 1
	}

	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = s.cfg.DefaultPageSize
	}
	if pageSize > s.cfg.MaxPageSize {
		pageSize = s.cfg.MaxPageSize
	}

	offset := (page - 1) * pageSize
	filter := repository.AdminUserListFilter{
		Query:  strings.TrimSpace(params.Query),
		Limit:  pageSize,
		Offset: offset,
	}

	rows, total, err := s.users.ListAdminOverview(ctx, filter)
	if err != nil {
		return OverviewResult{}, fmt.Errorf("list admin overview: %w", err)
	}

	items := make([]OverviewItem, 0, len(rows))
	if len(rows) == 0 {
		return OverviewResult{
			Items:                  items,
			Total:                  total,
			Page:                   page,
			PageSize:               pageSize,
			OnlineThresholdSeconds: int64(s.cfg.OnlineThreshold / time.Second),
		}, nil
	}

	userIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		userIDs = append(userIDs, row.ID)
	}

	recentMap := make(map[uint][]repository.UserRecentPromptRow)
	if s.prompts != nil && s.cfg.RecentPromptLimit > 0 {
		data, fetchErr := s.prompts.ListRecentByUsers(ctx, userIDs, s.cfg.RecentPromptLimit)
		if fetchErr != nil {
			s.logger.Warnw("list recent prompts failed", "error", fetchErr)
		} else {
			recentMap = data
		}
	}

	cutoff := time.Now().Add(-s.cfg.OnlineThreshold)

	for _, row := range rows {
		online := false
		if row.LastLoginAt != nil {
			online = !row.LastLoginAt.Before(cutoff)
		}
		promptTotals := PromptTotals{
			Total:     row.PromptTotal,
			Draft:     row.PromptDraft,
			Published: row.PromptPublished,
			Archived:  row.PromptArchived,
		}

		recents := make([]PromptSummary, 0)
		if prompts, ok := recentMap[row.ID]; ok {
			recents = make([]PromptSummary, 0, len(prompts))
			for _, promptRow := range prompts {
				recents = append(recents, PromptSummary{
					ID:        promptRow.PromptID,
					Topic:     promptRow.Topic,
					Status:    promptRow.Status,
					UpdatedAt: promptRow.UpdatedAt,
					CreatedAt: promptRow.CreatedAt,
				})
			}
		}

		items = append(items, OverviewItem{
			ID:             row.ID,
			Username:       row.Username,
			Email:          row.Email,
			AvatarURL:      row.AvatarURL,
			IsAdmin:        row.IsAdmin,
			LastLoginAt:    row.LastLoginAt,
			CreatedAt:      row.CreatedAt,
			UpdatedAt:      row.UpdatedAt,
			IsOnline:       online,
			PromptTotals:   promptTotals,
			LatestPromptAt: row.LatestPromptAt,
			RecentPrompts:  recents,
		})
	}

	return OverviewResult{
		Items:                  items,
		Total:                  total,
		Page:                   page,
		PageSize:               pageSize,
		OnlineThresholdSeconds: int64(s.cfg.OnlineThreshold / time.Second),
	}, nil
}
