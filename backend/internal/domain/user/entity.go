/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 20:38:45
 * @FilePath: \electron-go-app\backend\internal\domain\user\entity.go
 * @LastEditTime: 2025-10-08 20:38:50
 */
package user

import "time"

// User represents the persisted user entity in the system.
type User struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Username     string     `gorm:"size:64;uniqueIndex" json:"username"`
	Email        string     `gorm:"size:255;uniqueIndex" json:"email"`
	PasswordHash string     `gorm:"size:255" json:"-"`
	Settings     string     `gorm:"type:text" json:"settings"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
