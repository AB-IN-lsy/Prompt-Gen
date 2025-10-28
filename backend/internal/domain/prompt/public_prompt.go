package prompt

import "time"

// 公共库状态常量，标记审核流程的不同阶段。
const (
	PublicPromptStatusPending  = "pending"
	PublicPromptStatusApproved = "approved"
	PublicPromptStatusRejected = "rejected"
)

// PublicPrompt 描述公共 Prompt 库中的单条记录。
type PublicPrompt struct {
	ID               uint       `gorm:"primaryKey"`
	SourcePromptID   *uint      `gorm:"column:source_prompt_id"`
	AuthorUserID     uint       `gorm:"column:author_user_id;not null;index:idx_public_prompts_author"`
	Title            string     `gorm:"size:255;not null"`
	Topic            string     `gorm:"size:255;not null"`
	Summary          string     `gorm:"type:text;not null"`
	Body             string     `gorm:"type:text;not null"`
	Instructions     string     `gorm:"type:text;not null"`
	PositiveKeywords string     `gorm:"type:text;not null"`
	NegativeKeywords string     `gorm:"type:text;not null"`
	Tags             string     `gorm:"type:text;not null"`
	Model            string     `gorm:"size:64;not null"`
	Language         string     `gorm:"size:16;not null;default:'zh-CN'"`
	Status           string     `gorm:"size:16;not null;default:'pending';index:idx_public_prompts_status_created,priority:1"`
	ReviewerUserID   *uint      `gorm:"column:reviewer_user_id"`
	ReviewReason     string     `gorm:"size:255"`
	DownloadCount    uint       `gorm:"column:download_count;not null;default:0"`
	CreatedAt        time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;autoUpdateTime"`
	Reviewer         *UserBrief `gorm:"-"`
	Author           *UserBrief `gorm:"-"`
	LikeCount        uint       `gorm:"-"`
	IsLiked          bool       `gorm:"-"`
}

// TableName 返回对应的表名。
func (PublicPrompt) TableName() string {
	return "public_prompts"
}

// UserBrief 仅用于在公共库中展示作者或审核人的简要信息。
type UserBrief struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}
