/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-29 19:12:08
 * @FilePath: \electron-go-app\backend\internal\domain\prompt\public_prompt.go
 * @LastEditTime: 2025-10-30 22:12:49
 */
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
	ID               uint   `gorm:"primaryKey"`                                                     // 主键 ID
	SourcePromptID   *uint  `gorm:"column:source_prompt_id"`                                        // 原始个人 Prompt ID（若来源于投稿）
	AuthorUserID     uint   `gorm:"column:author_user_id;not null;index:idx_public_prompts_author"` // 作者用户 ID
	Title            string `gorm:"size:255;not null"`                                              // 展示标题
	Topic            string `gorm:"size:255;not null"`                                              // 主题关键词
	Summary          string `gorm:"type:text;not null"`                                             // 摘要内容
	Body             string `gorm:"type:text;not null"`                                             // Prompt 正文
	Instructions     string `gorm:"type:text;not null"`                                             // 补充要求
	PositiveKeywords string `gorm:"type:text;not null"`                                             // 正向关键词 JSON
	NegativeKeywords string `gorm:"type:text;not null"`                                             // 负向关键词 JSON
	Tags             string `gorm:"type:text;not null"`                                             // 标签 JSON
	Model            string `gorm:"size:64;not null"`                                               // 使用模型标识
	Language         string `gorm:"size:16;not null;default:'zh-CN'"`                               // 内容语言
	// 审核状态（pending/approved/rejected），同时作为多种排序索引的第一列
	Status         string     `gorm:"size:16;not null;default:'pending';index:idx_public_prompts_status_created,priority:1;index:idx_public_prompts_status_quality,priority:1;index:idx_public_prompts_status_download,priority:1"`
	ReviewerUserID *uint      `gorm:"column:reviewer_user_id"`                                                                      // 审核人 ID
	ReviewReason   string     `gorm:"size:255"`                                                                                     // 审核意见
	DownloadCount  uint       `gorm:"column:download_count;not null;default:0;index:idx_public_prompts_status_download,priority:2"` // 下载次数
	QualityScore   float64    `gorm:"column:quality_score;not null;default:0;index:idx_public_prompts_status_quality,priority:2"`   // 综合评分
	CreatedAt      time.Time  `gorm:"column:created_at;autoCreateTime;index:idx_public_prompts_status_created,priority:2"`          // 创建时间
	UpdatedAt      time.Time  `gorm:"column:updated_at;autoUpdateTime;index:idx_public_prompts_updated_at"`                         // 更新时间
	Reviewer       *UserBrief `gorm:"-"`                                                                                            // 审核人信息（运行时填充）
	Author         *UserBrief `gorm:"-"`                                                                                            // 作者信息（运行时填充）
	LikeCount      uint       `gorm:"-"`                                                                                            // 点赞数量（运行时填充）
	IsLiked        bool       `gorm:"-"`                                                                                            // 当前用户是否点赞
	VisitCount     uint64     `gorm:"-"`                                                                                            // 访问量（包含缓存增量）
}

// TableName 返回对应的表名。
func (PublicPrompt) TableName() string {
	return "public_prompts"
}

// UserBrief 仅用于在公共库中展示作者或审核人的简要信息。
type UserBrief struct {
	ID        uint   `json:"id"`         // 用户 ID
	Username  string `json:"username"`   // 显示名称
	Email     string `json:"email"`      // 邮箱（仅内部使用）
	AvatarURL string `json:"avatar_url"` // 头像链接
	Headline  string `json:"headline"`   // 主页口号
	Bio       string `json:"bio"`        // 个人简介
	Location  string `json:"location"`   // 地理位置/标签
	Website   string `json:"website"`    // 个人链接
	BannerURL string `json:"banner_url"` // 展示横幅
}
