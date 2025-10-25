package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// ModeLocal 表示当前运行在离线/本地模式。
	ModeLocal = "local"
	// ModeOnline 表示运行在默认的在线模式。
	ModeOnline = "online"

	defaultLocalUserID    = 1
	defaultLocalUsername  = "离线用户"
	defaultLocalEmail     = "offline@localhost"
	defaultLocalDBRelPath = "data/promptgen-local.db"
)

// RuntimeFlags 汇总运行期所需的模式与本地环境配置。
type RuntimeFlags struct {
	Mode  string
	Local LocalRuntime
}

// LocalRuntime 描述本地模式下需要的额外配置。
type LocalRuntime struct {
	DBPath   string
	UserID   uint
	Username string
	Email    string
	IsAdmin  bool
}

// LoadRuntimeFlags 读取环境变量，推导当前运行模式及本地模式参数。
func LoadRuntimeFlags() RuntimeFlags {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("APP_MODE")))
	if mode == "" {
		mode = ModeOnline
	}

	local := LocalRuntime{
		DBPath:   defaultLocalDBPath(),
		UserID:   defaultLocalUserID,
		Username: defaultLocalUsername,
		Email:    defaultLocalEmail,
		IsAdmin:  true,
	}

	if rawPath := strings.TrimSpace(os.Getenv("LOCAL_SQLITE_PATH")); rawPath != "" {
		local.DBPath = normalisePath(rawPath)
	}
	if rawID := strings.TrimSpace(os.Getenv("LOCAL_USER_ID")); rawID != "" {
		if parsed, err := strconv.ParseUint(rawID, 10, 32); err == nil && parsed > 0 {
			local.UserID = uint(parsed)
		}
	}
	if rawName := strings.TrimSpace(os.Getenv("LOCAL_USER_USERNAME")); rawName != "" {
		local.Username = rawName
	}
	if rawEmail := strings.TrimSpace(os.Getenv("LOCAL_USER_EMAIL")); rawEmail != "" {
		local.Email = rawEmail
	}
	if rawAdmin := strings.TrimSpace(os.Getenv("LOCAL_USER_ADMIN")); rawAdmin != "" {
		if parsed, err := strconv.ParseBool(rawAdmin); err == nil {
			local.IsAdmin = parsed
		}
	}

	return RuntimeFlags{
		Mode:  mode,
		Local: local,
	}
}

// defaultLocalDBPath 计算默认的本地数据库路径并返回绝对路径。
func defaultLocalDBPath() string {
	return normalisePath(defaultLocalDBRelPath)
}

// normalisePath 将路径展开为绝对路径，兼容 ~ 前缀与相对路径。
func normalisePath(raw string) string {
	if raw == "" {
		return raw
	}
	if strings.HasPrefix(raw, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			raw = filepath.Join(home, strings.TrimPrefix(raw, "~"))
		}
	}
	if filepath.IsAbs(raw) {
		return raw
	}
	if abs, err := filepath.Abs(raw); err == nil {
		return abs
	}
	return raw
}
