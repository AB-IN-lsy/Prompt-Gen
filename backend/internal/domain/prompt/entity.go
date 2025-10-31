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

// PromptCommentStatus 表示评论的审核状态。
const (
	PromptCommentStatusPending  = "pending"
	PromptCommentStatusApproved = "approved"
	PromptCommentStatusRejected = "rejected"
)

// PromptKeywordItem 映射 Prompt 中正负关键词 JSON 数组的元素结构。
type PromptKeywordItem struct {
	KeywordID uint   `json:"keyword_id,omitempty"` // 关键词主键（新建时可为空）
	Word      string `json:"word"`                 // 关键词文本
	Source    string `json:"source,omitempty"`     // 来源：manual/model/local
	Weight    int    `json:"weight,omitempty"`     // 权重（0-5）
}

// Prompt 表示用户保存的完整 Prompt 记录。
type Prompt struct {
	ID               uint       `gorm:"primaryKey"`                             // 自增主键。
	UserID           uint       `gorm:"index;not null"`                         // 关联的用户 ID。
	Topic            string     `gorm:"size:255;not null"`                      // Prompt 主题，如“React 面试”。
	Body             string     `gorm:"type:text;not null"`                     // Prompt 正文内容。
	Instructions     string     `gorm:"type:text;not null"`                     // 补充要求，用于指导生成模型的额外说明。
	PositiveKeywords string     `gorm:"type:text;not null"`                     // 正向关键词 JSON。
	NegativeKeywords string     `gorm:"type:text;not null"`                     // 负向关键词 JSON。
	Model            string     `gorm:"size:64;not null"`                       // 使用的大模型标识。
	Status           string     `gorm:"size:16;not null;default:'draft';index"` // 当前状态：draft/published/archived。
	Tags             string     `gorm:"type:text;not null"`                     // 自定义标签 JSON。
	IsFavorited      bool       `gorm:"not null;default:false"`                 // 是否收藏。
	LikeCount        uint       `gorm:"not null;default:0"`                     // 点赞数量。
	VisitCount       uint64     `gorm:"not null;default:0"`                     // 访问次数。
	PublishedAt      *time.Time // 最近发布的时间戳。
	CreatedAt        time.Time  // 创建时间。
	UpdatedAt        time.Time  // 最近更新时间。
	LatestVersionNo  int        `gorm:"not null;default:1"` // 最近发布版本号。
	IsLiked          bool       `gorm:"-"`                  // 当前用户是否点赞。
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

// PromptLike 记录用户对 Prompt 的点赞关系。
type PromptLike struct {
	PromptID  uint      `gorm:"primaryKey;column:prompt_id"`
	UserID    uint      `gorm:"primaryKey;column:user_id"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName 返回点赞关系使用的表名。
func (PromptLike) TableName() string {
	return "prompt_likes"
}

// PromptComment 记录 Prompt 下的评论，支持楼中楼与审核流程。
type PromptComment struct {
	ID             uint       `gorm:"primaryKey"`                                // 评论主键。
	PromptID       uint       `gorm:"not null;index:idx_prompt_comments_prompt"` // 关联 Prompt 编号。
	UserID         uint       `gorm:"not null;index:idx_prompt_comments_user"`   // 评论用户编号。
	ParentID       *uint      `gorm:"index:idx_prompt_comments_parent"`          // 直接父级评论编号。
	RootID         *uint      `gorm:"index:idx_prompt_comments_root"`            // 顶层评论编号，便于批量查询子楼层。
	Body           string     `gorm:"type:text;not null"`                        // 评论正文。
	Status         string     `gorm:"size:16;not null;default:'pending';index"`  // 审核状态：pending/approved/rejected。
	ReviewNote     string     `gorm:"type:text"`                                 // 审核备注，可选。
	ReviewerUserID *uint      `gorm:"index:idx_prompt_comments_reviewer"`        // 审核人编号。
	LikeCount      uint       `gorm:"not null;default:0"`                        // 点赞数量。
	ReplyCount     int        `gorm:"-"`                                         // 当前评论的可见子回复数量。
	Author         *UserBrief `gorm:"-"`                                         // 评论用户信息。
	IsLiked        bool       `gorm:"-"`                                         // 当前用户是否点赞。
	CreatedAt      time.Time  `gorm:"autoCreateTime"`                            // 创建时间。
	UpdatedAt      time.Time  `gorm:"autoUpdateTime"`                            // 最近更新时间。
}

// TableName 返回评论表名称。
func (PromptComment) TableName() string {
	return "prompt_comments"
}

// PromptCommentLike 记录用户对评论的点赞关系。
type PromptCommentLike struct {
	CommentID uint      `gorm:"primaryKey;column:comment_id"` // 评论主键。
	UserID    uint      `gorm:"primaryKey;column:user_id"`    // 点赞用户主键。
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName 返回评论点赞表名称。
func (PromptCommentLike) TableName() string {
	return "prompt_comment_likes"
}

// PromptVersion 记录历史版本，便于用户回滚。
type PromptVersion struct {
	ID               uint      `gorm:"primaryKey"`                                          // 自增主键。
	PromptID         uint      `gorm:"not null;index;index:idx_prompt_versions,priority:1"` // 关联 Prompt。
	VersionNo        int       `gorm:"not null;index:idx_prompt_versions,priority:2"`       // 版本号。
	Body             string    `gorm:"type:text;not null"`                                  // Prompt 正文快照。
	Instructions     string    `gorm:"type:text;not null"`                                  // 补充要求快照。
	PositiveKeywords string    `gorm:"type:text;not null"`                                  // 正向关键词快照。
	NegativeKeywords string    `gorm:"type:text;not null"`                                  // 负向关键词快照。
	Model            string    `gorm:"size:64;not null"`                                    // 生成使用的模型。
	CreatedAt        time.Time // 版本创建时间。
}
