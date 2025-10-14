package prompt

import "time"

// KeywordSource 定义关键词的来源标签，便于前端展示及去重策略。
const (
	KeywordSourceManual = "manual"
	KeywordSourceModel  = "model"
	KeywordSourceLocal  = "local"
)

// KeywordPolarity 描述关键词的正负向分类。
const (
	KeywordPolarityPositive = "positive"
	KeywordPolarityNegative = "negative"
)

// PromptStatus 表示 Prompt 的当前状态。
const (
	PromptStatusDraft     = "draft"
	PromptStatusPublished = "published"
	PromptStatusArchived  = "archived"
)

// PromptKeywordItem 映射 Prompt 中正负关键词 JSON 数组的元素结构。
type PromptKeywordItem struct {
	KeywordID uint   `json:"keyword_id,omitempty"`
	Word      string `json:"word"`
	Source    string `json:"source,omitempty"`
	Weight    int    `json:"weight,omitempty"`
}

// Prompt 表示用户保存的完整 Prompt 记录。
type Prompt struct {
	ID               uint       `gorm:"primaryKey"`                             // 自增主键。
	UserID           uint       `gorm:"index;not null"`                         // 关联的用户 ID。
	Topic            string     `gorm:"size:255;not null"`                      // Prompt 主题，如“React 面试”。
	Body             string     `gorm:"type:text;not null"`                     // Prompt 正文内容。
	PositiveKeywords string     `gorm:"type:text;not null"`                     // 正向关键词 JSON。
	NegativeKeywords string     `gorm:"type:text;not null"`                     // 负向关键词 JSON。
	Model            string     `gorm:"size:64;not null"`                       // 使用的大模型标识。
	Status           string     `gorm:"size:16;not null;default:'draft';index"` // 当前状态：draft/published/archived。
	Tags             string     `gorm:"type:text;not null"`                     // 自定义标签 JSON。
	PublishedAt      *time.Time // 最近发布的时间戳。
	CreatedAt        time.Time  // 创建时间。
	UpdatedAt        time.Time  // 最近更新时间。
	LatestVersionNo  int        `gorm:"not null;default:1"` // 最近发布版本号。
}

// Keyword 表示主题下的单个关键词，包含正负向、权重与来源信息。
type Keyword struct {
	ID        uint      `gorm:"primaryKey"`                                                                                                    // 自增主键。
	UserID    uint      `gorm:"not null;index:idx_keywords_user_topic,priority:1;uniqueIndex:uk_keywords_user_topic_word,priority:1"`          // 关联用户 ID。
	Topic     string    `gorm:"size:255;not null;index:idx_keywords_user_topic,priority:2;uniqueIndex:uk_keywords_user_topic_word,priority:2"` // 所属主题。
	Word      string    `gorm:"size:255;not null;uniqueIndex:uk_keywords_user_topic_word,priority:3"`                                          // 关键词文本。
	Polarity  string    `gorm:"size:16;not null;index:idx_keywords_user_word_topic,priority:2"`                                                // 正向/负向。
	Source    string    `gorm:"size:16;not null"`                                                                                              // 来源：manual/model/local。
	Weight    int       `gorm:"default:0"`                                                                                                     // 权重，用于排序。
	Language  string    `gorm:"size:16;not null;default:'zh'"`                                                                                 // 语言标识。
	CreatedAt time.Time // 创建时间。
	UpdatedAt time.Time // 更新时间。
}

// PromptKeyword 用于建立 Prompt 与 Keyword 之间的多对多关系。
type PromptKeyword struct {
	PromptID  uint      `gorm:"primaryKey;index:idx_prompt_keyword_prompt"`  // Prompt 主键。
	KeywordID uint      `gorm:"primaryKey;index:idx_prompt_keyword_keyword"` // Keyword 主键。
	Relation  string    `gorm:"size:16;not null"`                            // 关联类型：positive/negative。
	CreatedAt time.Time // 关系创建时间。
}

// PromptVersion 记录历史版本，便于用户回滚。
type PromptVersion struct {
	ID               uint      `gorm:"primaryKey"`                                          // 自增主键。
	PromptID         uint      `gorm:"not null;index;index:idx_prompt_versions,priority:1"` // 关联 Prompt。
	VersionNo        int       `gorm:"not null;index:idx_prompt_versions,priority:2"`       // 版本号。
	Body             string    `gorm:"type:text;not null"`                                  // Prompt 正文快照。
	PositiveKeywords string    `gorm:"type:text;not null"`                                  // 正向关键词快照。
	NegativeKeywords string    `gorm:"type:text;not null"`                                  // 负向关键词快照。
	Model            string    `gorm:"size:64;not null"`                                    // 生成使用的模型。
	CreatedAt        time.Time // 版本创建时间。
}
