package bootstrapdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	changelog "electron-go-app/backend/internal/domain/changelog"
	promptdomain "electron-go-app/backend/internal/domain/prompt"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	envDataDir              = "LOCAL_BOOTSTRAP_DATA_DIR"
	defaultBootstrapDataDir = "backend/data/bootstrap"
	publicPromptFilename    = "public_prompts.json"
	changelogFilename       = "changelog_entries.json"
)

// Options 描述预置数据导入所需的可选参数。
type Options struct {
	DataDir        string
	AuthorUserID   uint
	ReviewerUserID *uint
	Logger         *zap.SugaredLogger
}

// SeedLocalDatabase 在本地模式下向空数据库导入公共 Prompt 与更新日志。
func SeedLocalDatabase(ctx context.Context, db *gorm.DB, opts Options) error {
	if db == nil {
		return errors.New("db is nil")
	}

	if opts.DataDir == "" {
		opts.DataDir = ResolveDataDir()
	}

	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	if opts.AuthorUserID == 0 {
		logger.Infow("skip bootstrap because author id missing")
		return nil
	}

	if err := seedPublicPrompts(ctx, db, opts, logger); err != nil {
		return err
	}
	if err := seedChangelogEntries(ctx, db, opts, logger); err != nil {
		return err
	}

	return nil
}

// ResolveDataDir 解析预置数据所在目录。
func ResolveDataDir() string {
	raw := strings.TrimSpace(os.Getenv(envDataDir))
	if raw == "" {
		return defaultBootstrapDataDir
	}
	return raw
}

type seedKeyword struct {
	Word   string   `json:"word"`
	Weight *float64 `json:"weight"`
	Source string   `json:"source"`
}

type publicPromptSeed struct {
	Title            string        `json:"title"`
	Topic            string        `json:"topic"`
	Summary          string        `json:"summary"`
	Body             string        `json:"body"`
	Instructions     string        `json:"instructions"`
	PositiveKeywords []seedKeyword `json:"positive_keywords"`
	NegativeKeywords []seedKeyword `json:"negative_keywords"`
	Tags             []string      `json:"tags"`
	Model            string        `json:"model"`
	Language         string        `json:"language"`
	Status           string        `json:"status"`
	ReviewReason     string        `json:"review_reason"`
	SourcePromptID   *uint         `json:"source_prompt_id"`
	DownloadCount    *uint         `json:"download_count"`
	AuthorUserID     *uint         `json:"author_user_id"`
	ReviewerUserID   *uint         `json:"reviewer_user_id"`
}

type changelogSeed struct {
	Locale      string   `json:"locale"`
	Badge       string   `json:"badge"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Items       []string `json:"items"`
	PublishedAt string   `json:"published_at"`
	AuthorID    *uint    `json:"author_id"`
}

// seedPublicPrompts 将公共 Prompt 预置数据写入数据库，仅在当前表为空时执行。
func seedPublicPrompts(ctx context.Context, db *gorm.DB, opts Options, logger *zap.SugaredLogger) error {
	path := filepath.Join(opts.DataDir, publicPromptFilename)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		logger.Infow("public prompt seed not found, skip", "path", path)
		return nil
	}
	if err != nil {
		return fmt.Errorf("read public prompt seed: %w", err)
	}

	var seeds []publicPromptSeed
	if err := json.Unmarshal(raw, &seeds); err != nil {
		return fmt.Errorf("parse public prompt seed: %w", err)
	}
	if len(seeds) == 0 {
		logger.Infow("public prompt seed empty, skip")
		return nil
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		inserted := 0
		updated := 0
		for idx, item := range seeds {
			authorID := opts.AuthorUserID
			if item.AuthorUserID != nil && *item.AuthorUserID != 0 {
				authorID = *item.AuthorUserID
			}
			if authorID == 0 {
				return fmt.Errorf("seed public prompt missing author id at index %d", idx)
			}

			reviewerID := opts.ReviewerUserID
			if item.ReviewerUserID != nil {
				reviewerID = item.ReviewerUserID
			}

			positiveJSON, err := json.Marshal(item.PositiveKeywords)
			if err != nil {
				return fmt.Errorf("encode positive keywords at index %d: %w", idx, err)
			}
			negativeJSON, err := json.Marshal(item.NegativeKeywords)
			if err != nil {
				return fmt.Errorf("encode negative keywords at index %d: %w", idx, err)
			}
			tagsJSON, err := json.Marshal(item.Tags)
			if err != nil {
				return fmt.Errorf("encode tags at index %d: %w", idx, err)
			}

			entity := promptdomain.PublicPrompt{
				SourcePromptID:   item.SourcePromptID,
				AuthorUserID:     authorID,
				Title:            strings.TrimSpace(item.Title),
				Topic:            strings.TrimSpace(item.Topic),
				Summary:          strings.TrimSpace(item.Summary),
				Body:             item.Body,
				Instructions:     item.Instructions,
				PositiveKeywords: string(positiveJSON),
				NegativeKeywords: string(negativeJSON),
				Tags:             string(tagsJSON),
				Model:            strings.TrimSpace(item.Model),
				Language:         defaultLanguage(item.Language),
				Status:           defaultStatus(item.Status),
				ReviewReason:     strings.TrimSpace(item.ReviewReason),
			}
			if reviewerID != nil {
				entity.ReviewerUserID = reviewerID
			}
			if item.DownloadCount != nil {
				entity.DownloadCount = *item.DownloadCount
			}

			var existing promptdomain.PublicPrompt
			err = tx.Where("author_user_id = ? AND topic = ?", authorID, entity.Topic).First(&existing).Error
			switch {
			case err == nil:
				existing.Title = entity.Title
				existing.Summary = entity.Summary
				existing.Body = entity.Body
				existing.Instructions = entity.Instructions
				existing.PositiveKeywords = entity.PositiveKeywords
				existing.NegativeKeywords = entity.NegativeKeywords
				existing.Tags = entity.Tags
				existing.Model = entity.Model
				existing.Language = entity.Language
				existing.Status = entity.Status
				existing.ReviewReason = entity.ReviewReason
				existing.SourcePromptID = entity.SourcePromptID
				existing.ReviewerUserID = entity.ReviewerUserID
				if item.DownloadCount != nil {
					existing.DownloadCount = *item.DownloadCount
				}
				existing.UpdatedAt = time.Now()
				if saveErr := tx.Save(&existing).Error; saveErr != nil {
					return fmt.Errorf("update public prompt at index %d: %w", idx, saveErr)
				}
				updated++
			case errors.Is(err, gorm.ErrRecordNotFound):
				if createErr := tx.Create(&entity).Error; createErr != nil {
					return fmt.Errorf("insert public prompt at index %d: %w", idx, createErr)
				}
				inserted++
			default:
				return fmt.Errorf("query public prompt at index %d: %w", idx, err)
			}
		}
		logger.Infow("同步公共 Prompt 数据完成", "inserted", inserted, "updated", updated, "total", len(seeds))
		return nil
	})
}

func seedChangelogEntries(ctx context.Context, db *gorm.DB, opts Options, logger *zap.SugaredLogger) error {
	path := filepath.Join(opts.DataDir, changelogFilename)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		logger.Infow("changelog seed not found, skip", "path", path)
		return nil
	}
	if err != nil {
		return fmt.Errorf("read changelog seed: %w", err)
	}

	var seeds []changelogSeed
	if err := json.Unmarshal(raw, &seeds); err != nil {
		return fmt.Errorf("parse changelog seed: %w", err)
	}
	if len(seeds) == 0 {
		logger.Infow("changelog seed empty, skip")
		return nil
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		inserted := 0
		updated := 0
		for idx, item := range seeds {
			publishedAt, err := time.Parse(time.DateOnly, strings.TrimSpace(item.PublishedAt))
			if err != nil {
				return fmt.Errorf("parse published_at at index %d: %w", idx, err)
			}

			authorID := opts.AuthorUserID
			if item.AuthorID != nil && *item.AuthorID != 0 {
				authorID = *item.AuthorID
			}

			entry := changelog.Entry{
				Locale:      defaultLocale(item.Locale),
				Badge:       strings.TrimSpace(item.Badge),
				Title:       strings.TrimSpace(item.Title),
				Summary:     strings.TrimSpace(item.Summary),
				Items:       jsonList(item.Items),
				PublishedAt: publishedAt,
			}
			if authorID != 0 {
				entry.AuthorID = &authorID
			}

			var existing changelog.Entry
			err = tx.Where("locale = ? AND title = ?", entry.Locale, entry.Title).First(&existing).Error
			switch {
			case err == nil:
				existing.Badge = entry.Badge
				existing.Title = entry.Title
				existing.Summary = entry.Summary
				existing.Items = entry.Items
				existing.PublishedAt = entry.PublishedAt
				if authorID != 0 {
					existing.AuthorID = entry.AuthorID
				}
				existing.UpdatedAt = time.Now()
				if saveErr := tx.Save(&existing).Error; saveErr != nil {
					return fmt.Errorf("update changelog at index %d: %w", idx, saveErr)
				}
				updated++
			case errors.Is(err, gorm.ErrRecordNotFound):
				if createErr := tx.Create(&entry).Error; createErr != nil {
					return fmt.Errorf("insert changelog at index %d: %w", idx, createErr)
				}
				inserted++
			default:
				return fmt.Errorf("query changelog at index %d: %w", idx, err)
			}
		}
		logger.Infow("同步更新日志数据完成", "inserted", inserted, "updated", updated, "total", len(seeds))
		return nil
	})
}

type ExportOptions struct {
	OutputDir string
	Logger    *zap.SugaredLogger
}

// ExportSnapshot 会从当前数据库中读取公共 Prompt 与更新日志，并生成可供离线模式使用的 JSON 文件。
func ExportSnapshot(ctx context.Context, db *gorm.DB, opts ExportOptions) error {
	if db == nil {
		return errors.New("db is nil")
	}
	if opts.OutputDir == "" {
		opts.OutputDir = ResolveDataDir()
	}
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	if err := ensureDir(opts.OutputDir); err != nil {
		return fmt.Errorf("ensure output dir: %w", err)
	}
	if err := exportPublicPrompts(ctx, db, opts.OutputDir); err != nil {
		return err
	}
	if err := exportChangelogEntries(ctx, db, opts.OutputDir); err != nil {
		return err
	}
	logger.Infow("exported offline snapshot", "output_dir", opts.OutputDir)
	return nil
}

// exportPublicPrompts 将数据库中的公共 Prompt 数据导出为 JSON 文件。
func exportPublicPrompts(ctx context.Context, db *gorm.DB, outputDir string) error {
	var prompts []promptdomain.PublicPrompt
	if err := db.WithContext(ctx).Order("id ASC").Find(&prompts).Error; err != nil {
		return fmt.Errorf("query public prompts: %w", err)
	}
	exportItems := make([]map[string]interface{}, 0, len(prompts))
	for _, item := range prompts {
		entry := map[string]interface{}{
			"title":             item.Title,
			"topic":             item.Topic,
			"summary":           item.Summary,
			"body":              item.Body,
			"instructions":      item.Instructions,
			"positive_keywords": decodeKeywordList(item.PositiveKeywords),
			"negative_keywords": decodeKeywordList(item.NegativeKeywords),
			"tags":              decodeStringList(item.Tags),
			"model":             item.Model,
			"language":          item.Language,
			"status":            item.Status,
			"review_reason":     strings.TrimSpace(item.ReviewReason),
		}
		if item.SourcePromptID != nil {
			entry["source_prompt_id"] = item.SourcePromptID
		}
		if item.AuthorUserID != 0 {
			entry["author_user_id"] = item.AuthorUserID
		}
		if item.ReviewerUserID != nil {
			entry["reviewer_user_id"] = item.ReviewerUserID
		}
		if item.DownloadCount > 0 {
			entry["download_count"] = item.DownloadCount
		}
		exportItems = append(exportItems, entry)
	}
	path := filepath.Join(outputDir, publicPromptFilename)
	return writeJSON(path, exportItems)
}

// defaultLanguage 在缺省时回退到中文语言标识。
func defaultLanguage(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "zh-CN"
	}
	return trimmed
}

// defaultStatus 在缺省时回退到已审核通过状态。
func defaultStatus(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return promptdomain.PublicPromptStatusApproved
	}
	return trimmed
}

// defaultLocale 在缺省时回退到中文区域设置。
func defaultLocale(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "zh-CN"
	}
	return trimmed
}

// jsonList 将字符串数组编码为 JSON 字节切片。
func jsonList(items []string) []byte {
	data, err := json.Marshal(items)
	if err != nil {
		return []byte("[]")
	}
	return data
}

// exportChangelogEntries 将数据库中的更新日志导出为 JSON 文件。
func exportChangelogEntries(ctx context.Context, db *gorm.DB, outputDir string) error {
	var entries []changelog.Entry
	if err := db.WithContext(ctx).Order("published_at DESC, id DESC").Find(&entries).Error; err != nil {
		return fmt.Errorf("query changelog entries: %w", err)
	}
	exportItems := make([]map[string]interface{}, 0, len(entries))
	for _, item := range entries {
		entry := map[string]interface{}{
			"locale":       item.Locale,
			"badge":        item.Badge,
			"title":        item.Title,
			"summary":      item.Summary,
			"items":        decodeChangelogItems(item.Items),
			"published_at": item.PublishedAt.Format(time.DateOnly),
		}
		if item.AuthorID != nil {
			entry["author_id"] = *item.AuthorID
		}
		exportItems = append(exportItems, entry)
	}
	path := filepath.Join(outputDir, changelogFilename)
	return writeJSON(path, exportItems)
}

// decodeKeywordList 解析存储在文本字段中的关键词 JSON。
func decodeKeywordList(raw string) []seedKeyword {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var structured []seedKeyword
	if err := json.Unmarshal([]byte(trimmed), &structured); err == nil {
		return structured
	}
	var generic []map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &generic); err == nil {
		result := make([]seedKeyword, 0, len(generic))
		for _, item := range generic {
			word, _ := item["word"].(string)
			word = strings.TrimSpace(word)
			if word == "" {
				continue
			}
			var weightPtr *float64
			if weight, ok := item["weight"].(float64); ok {
				weightPtr = new(float64)
				*weightPtr = weight
			}
			source := ""
			if rawSource, ok := item["source"].(string); ok {
				source = strings.TrimSpace(rawSource)
			}
			result = append(result, seedKeyword{
				Word:   word,
				Weight: weightPtr,
				Source: source,
			})
		}
		return result
	}
	var plain []string
	if err := json.Unmarshal([]byte(trimmed), &plain); err == nil {
		result := make([]seedKeyword, 0, len(plain))
		for _, word := range plain {
			if candidate := strings.TrimSpace(word); candidate != "" {
				result = append(result, seedKeyword{Word: candidate})
			}
		}
		return result
	}
	return nil
}

// decodeStringList 将文本字段解析为字符串数组。
func decodeStringList(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(trimmed), &arr); err == nil {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if candidate := strings.TrimSpace(item); candidate != "" {
				result = append(result, candidate)
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	}
	return nil
}

// decodeChangelogItems 将 changelog 项中的 JSON 转换为字符串数组。
func decodeChangelogItems(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	var items []string
	if err := json.Unmarshal(data, &items); err == nil {
		result := make([]string, 0, len(items))
		for _, item := range items {
			if candidate := strings.TrimSpace(item); candidate != "" {
				result = append(result, candidate)
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	}
	return nil
}

// writeJSON 将数据格式化后写入指定文件。
func writeJSON(path string, payload interface{}) error {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(path, encoded, 0o644)
}

// ensureDir 保证目录存在，用于写入导出文件。
func ensureDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
