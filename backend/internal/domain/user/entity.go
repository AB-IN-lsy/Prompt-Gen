/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 20:38:45
 * @FilePath: \electron-go-app\backend\internal\domain\user\entity.go
 * @LastEditTime: 2025-10-08 20:38:50
 */
package user

import (
	"encoding/json"
	"time"
)

// User represents the persisted user entity in the system.
type User struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Username     string     `gorm:"size:64;uniqueIndex" json:"username"`
	Email        string     `gorm:"size:255;uniqueIndex" json:"email"`
	AvatarURL    string     `gorm:"size:512" json:"avatar_url"` // 用户头像的公开访问地址
	PasswordHash string     `gorm:"size:255" json:"-"`
	Settings     string     `gorm:"type:text" json:"settings"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Settings 描述用户可自定义的配置项，会以 JSON 字符串形式持久化在 User.Settings 字段中。
type Settings struct {
	PreferredModel string `json:"preferred_model"`
	SyncEnabled    bool   `json:"sync_enabled"`
}

// DefaultSettings 返回默认的用户设置。
func DefaultSettings() Settings {
	return Settings{
		PreferredModel: "deepseek",
		SyncEnabled:    false,
	}
}

// ParseSettings 从 JSON 字符串解析用户设置，若为空则返回默认值。
func ParseSettings(raw string) (Settings, error) {
	if raw == "" {
		return DefaultSettings(), nil
	}
	var s Settings
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return Settings{}, err
	}
	return s, nil
}

// SettingsJSON 将设置对象编码为 JSON 字符串，供数据库持久化使用。
func SettingsJSON(s Settings) (string, error) {
	if s.PreferredModel == "" {
		s.PreferredModel = DefaultSettings().PreferredModel
	}
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
