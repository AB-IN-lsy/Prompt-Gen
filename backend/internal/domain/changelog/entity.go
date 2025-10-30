/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-12 11:02:00
 * @FilePath: \electron-go-app\backend\internal\domain\changelog\entity.go
 * @LastEditTime: 2025-10-12 11:02:00
 */
package changelog

import (
	"time"

	user "electron-go-app/backend/internal/domain/user"

	"gorm.io/datatypes"
)

// Entry 对应应用内的更新日志记录。
type Entry struct {
	ID          uint           `gorm:"primaryKey" json:"id"`                                                         // 主键 ID
	Locale      string         `gorm:"size:16;index:idx_changelog_locale_date,priority:1" json:"locale"`             // 语言标识（zh-CN/en 等）
	Badge       string         `gorm:"size:64" json:"badge"`                                                         // 徽章标签（如“新增”）
	Title       string         `gorm:"size:255" json:"title"`                                                        // 日志标题
	Summary     string         `gorm:"type:text" json:"summary"`                                                     // 简要描述
	Items       datatypes.JSON `gorm:"type:json" json:"items"`                                                       // 详细内容列表（JSON）
	PublishedAt time.Time      `gorm:"type:datetime;index:idx_changelog_locale_date,priority:2" json:"published_at"` // 发布时间
	AuthorID    *uint          `json:"author_id"`                                                                    // 作者用户 ID
	Author      *user.User     `gorm:"constraint:OnDelete:SET NULL" json:"-"`                                        // 作者信息（删除作者时置空）
	CreatedAt   time.Time      `json:"created_at"`                                                                   // 创建时间
	UpdatedAt   time.Time      `json:"updated_at"`                                                                   // 更新时间
}

// TableName 指定数据库表名。
func (Entry) TableName() string {
	return "changelog_entries"
}
