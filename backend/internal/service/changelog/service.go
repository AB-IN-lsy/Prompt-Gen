/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-12 11:15:48
 * @FilePath: \electron-go-app\backend\internal\service\changelog\service.go
 * @LastEditTime: 2025-10-12 11:15:48
 */
package changelog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "electron-go-app/backend/internal/domain/changelog"
	"electron-go-app/backend/internal/repository"

	"gorm.io/gorm"
)

// ErrEntryNotFound 表示指定日志不存在。
var ErrEntryNotFound = errors.New("changelog entry not found")

// Service 封装更新日志的读写逻辑。
type Service struct {
	entries *repository.ChangelogRepository
}

// NewService 构造日志服务。
func NewService(entries *repository.ChangelogRepository) *Service {
	return &Service{entries: entries}
}

// Entry 表示返回给前端的日志信息。
type Entry struct {
	ID          uint      `json:"id"`
	Locale      string    `json:"locale"`
	Badge       string    `json:"badge"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	Items       []string  `json:"items"`
	PublishedAt string    `json:"published_at"`
	AuthorID    *uint     `json:"author_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateEntryParams 描述新增/更新日志时允许填写的字段。
type CreateEntryParams struct {
	Locale      string
	Badge       string
	Title       string
	Summary     string
	Items       []string
	PublishedAt string // yyyy-MM-dd
	AuthorID    *uint
}

// UpdateEntryParams 与 CreateEntryParams 相同，目前不额外扩展。
type UpdateEntryParams = CreateEntryParams

// ListEntries 返回指定语言（默认为 en）的所有日志记录。
func (s *Service) ListEntries(ctx context.Context, locale string) ([]Entry, error) {
	lang := normaliseLocale(locale)
	rawEntries, err := s.entries.ListByLocale(ctx, lang, 0)
	if err != nil {
		return nil, fmt.Errorf("list changelog entries: %w", err)
	}
	if len(rawEntries) == 0 && lang != "en" {
		rawEntries, err = s.entries.ListByLocale(ctx, "en", 0)
		if err != nil {
			return nil, fmt.Errorf("list fallback changelog entries: %w", err)
		}
	}

	result := make([]Entry, 0, len(rawEntries))
	for _, item := range rawEntries {
		entry, err := toEntry(item)
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}

	return result, nil
}

// CreateEntry 新增一条日志。
func (s *Service) CreateEntry(ctx context.Context, params CreateEntryParams) (Entry, error) {
	entry, err := s.persist(ctx, 0, params)
	if err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// UpdateEntry 更新指定日志。
func (s *Service) UpdateEntry(ctx context.Context, id uint, params UpdateEntryParams) (Entry, error) {
	entry, err := s.persist(ctx, id, params)
	if err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// DeleteEntry 删除指定日志。
func (s *Service) DeleteEntry(ctx context.Context, id uint) error {
	if err := s.entries.Delete(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEntryNotFound
		}
		return fmt.Errorf("delete entry: %w", err)
	}
	return nil
}

func (s *Service) persist(ctx context.Context, id uint, params CreateEntryParams) (Entry, error) {
	if len(params.Items) == 0 {
		return Entry{}, fmt.Errorf("items cannot be empty")
	}

	locale := normaliseLocale(params.Locale)
	if locale == "" {
		locale = "en"
	}

	publishedAt, err := parseDate(params.PublishedAt)
	if err != nil {
		return Entry{}, err
	}

	itemsJSON, err := json.Marshal(params.Items)
	if err != nil {
		return Entry{}, fmt.Errorf("encode items: %w", err)
	}

	var model *domain.Entry
	if id != 0 {
		current, err := s.entries.FindByID(ctx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return Entry{}, ErrEntryNotFound
			}
			return Entry{}, fmt.Errorf("load entry: %w", err)
		}
		model = current
	} else {
		model = &domain.Entry{}
	}

	model.Locale = locale
	model.Badge = strings.TrimSpace(params.Badge)
	model.Title = strings.TrimSpace(params.Title)
	model.Summary = strings.TrimSpace(params.Summary)
	model.Items = itemsJSON
	model.PublishedAt = publishedAt
	model.AuthorID = params.AuthorID

	var saveErr error
	if id == 0 {
		saveErr = s.entries.Create(ctx, model)
	} else {
		saveErr = s.entries.Update(ctx, model)
	}
	if saveErr != nil {
		return Entry{}, fmt.Errorf("persist entry: %w", saveErr)
	}

	return toEntry(*model)
}

func parseDate(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("published_at cannot be empty")
	}
	layouts := []string{time.DateOnly, "2006-01-02", time.RFC3339}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, trimmed); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid published_at format: %s", raw)
}

func toEntry(item domain.Entry) (Entry, error) {
	var items []string
	if len(item.Items) > 0 {
		if err := json.Unmarshal(item.Items, &items); err != nil {
			return Entry{}, fmt.Errorf("decode items: %w", err)
		}
	}
	return Entry{
		ID:          item.ID,
		Locale:      item.Locale,
		Badge:       item.Badge,
		Title:       item.Title,
		Summary:     item.Summary,
		Items:       items,
		PublishedAt: item.PublishedAt.Format(time.DateOnly),
		AuthorID:    item.AuthorID,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}, nil
}

func normaliseLocale(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "en"
	}
	if strings.EqualFold(trimmed, "zh") {
		return "zh-CN"
	}
	if strings.EqualFold(trimmed, "en-US") || strings.EqualFold(trimmed, "en-GB") {
		return "en"
	}
	return trimmed
}
