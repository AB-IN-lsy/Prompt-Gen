/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 20:38:45
 * @FilePath: \electron-go-app\backend\internal\domain\user\entity.go
 * @LastEditTime: 2025-10-11 01:36:29
 */
package user

import (
	"encoding/json"
	"time"
)

// User represents the persisted user entity in the system.
type User struct {
	ID              uint       `gorm:"primaryKey" json:"id"`                // 自增主键
	Username        string     `gorm:"size:64;uniqueIndex" json:"username"` // 登录/展示用的唯一用户名
	Email           string     `gorm:"size:255;uniqueIndex" json:"email"`   // 登录邮箱（唯一）
	AvatarURL       string     `gorm:"size:512" json:"avatar_url"`          // 用户头像的公开访问地址
	PasswordHash    string     `gorm:"size:255" json:"-"`                   // Bcrypt 等算法生成的密码哈希
	IsAdmin         bool       `gorm:"default:false" json:"is_admin"`       // 管理员标记，仅限内部控制面
	Settings        string     `gorm:"type:text" json:"settings"`           // JSON 字符串，存放偏好设置
	LastLoginAt     *time.Time `json:"last_login_at"`                       // 上次登录时间，可为空
	EmailVerifiedAt *time.Time `json:"email_verified_at"`                   // 邮箱通过验证的时间，可为空
	CreatedAt       time.Time  `json:"created_at"`                          // 创建时间戳（gorm 自动维护）
	UpdatedAt       time.Time  `json:"updated_at"`                          // 更新时间戳（gorm 自动维护）
}

// EmailVerificationToken 记录邮箱验证所需的一次性令牌。
type EmailVerificationToken struct {
	ID         uint       `gorm:"primaryKey"`                 // 主键
	UserID     uint       `gorm:"uniqueIndex:user_id"`        // 关联的用户 ID（唯一），确保每次只有一条生效记录
	Token      string     `gorm:"size:128;uniqueIndex:token"` // 发往邮箱的一次性验证令牌
	ExpiresAt  time.Time  `gorm:"index"`                      // 过期时间
	ConsumedAt *time.Time // 实际使用时间（已验证时记录）
	CreatedAt  time.Time  // 创建时间
	UpdatedAt  time.Time  // 更新时间

	User User `gorm:"constraint:OnDelete:CASCADE"`
}

// UserModelCredential 保存用户为特定模型配置的访问凭据。
type UserModelCredential struct {
	ID             uint       `gorm:"primaryKey" json:"id"`                     // 主键
	UserID         uint       `gorm:"index" json:"user_id"`                     // 所属用户 ID
	Provider       string     `gorm:"size:64;index" json:"provider"`            // 模型服务提供方，如 openai、deepseek
	ModelKey       string     `gorm:"size:128" json:"model_key"`                // 用户自定义的模型键（用于前端偏好引用）
	DisplayName    string     `gorm:"size:128" json:"display_name"`             // 前端展示名称
	BaseURL        string     `gorm:"size:512" json:"base_url"`                 // 可选，自定义 API Base URL
	APIKeyCipher   []byte     `gorm:"column:api_key_cipher;type:blob" json:"-"` // 加密后的 API Key（二进制存储）
	ExtraConfig    string     `gorm:"type:text" json:"extra_config"`            // 扩展配置（JSON 字符串）
	Status         string     `gorm:"size:16" json:"status"`                    // 状态：enabled/disabled
	LastVerifiedAt *time.Time `json:"last_verified_at"`                         // 最近一次连通性校验时间
	CreatedAt      time.Time  `json:"created_at"`                               // 创建时间（gorm 自动维护）
	UpdatedAt      time.Time  `json:"updated_at"`                               // 更新时间（gorm 自动维护）
}

// TableName overrides default naming for clarity when syncing with PRD。
func (UserModelCredential) TableName() string {
	return "user_model_credentials"
}

// Settings 描述用户可自定义的配置项，会以 JSON 字符串形式持久化在 User.Settings 字段中。
type Settings struct {
	PreferredModel   string `json:"preferred_model"`
	EnableAnimations bool   `json:"enable_animations"`
}

// DefaultSettings 返回默认的用户设置。
func DefaultSettings() Settings {
	return Settings{
		PreferredModel:   "deepseek",
		EnableAnimations: true,
	}
}

// ParseSettings 从 JSON 字符串解析用户设置，若为空则返回默认值。
func ParseSettings(raw string) (Settings, error) {
	if raw == "" {
		return DefaultSettings(), nil
	}
	type payload struct {
		PreferredModel   string `json:"preferred_model"`
		EnableAnimations *bool  `json:"enable_animations"`
	}
	var dto payload
	if err := json.Unmarshal([]byte(raw), &dto); err != nil {
		return Settings{}, err
	}
	settings := DefaultSettings()
	if dto.PreferredModel != "" {
		settings.PreferredModel = dto.PreferredModel
	}
	if dto.EnableAnimations != nil {
		settings.EnableAnimations = *dto.EnableAnimations
	}
	return settings, nil
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
