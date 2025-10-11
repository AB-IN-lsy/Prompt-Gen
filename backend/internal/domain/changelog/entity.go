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
	ID          uint           `gorm:"primaryKey" json:"id"`
	Locale      string         `gorm:"size:16;index:idx_changelog_locale_date,priority:1" json:"locale"`
	Badge       string         `gorm:"size:64" json:"badge"`
	Title       string         `gorm:"size:255" json:"title"`
	Summary     string         `gorm:"type:text" json:"summary"`
	Items       datatypes.JSON `gorm:"type:json" json:"items"`
	PublishedAt time.Time      `gorm:"type:datetime;index:idx_changelog_locale_date,priority:2" json:"published_at"`
	AuthorID    *uint          `json:"author_id"`
	Author      *user.User     `gorm:"constraint:OnDelete:SET NULL" json:"-"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// TableName 指定数据库表名。
func (Entry) TableName() string {
	return "changelog_entries"
}
