/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-08 17:58:06
 * @FilePath: \electron-go-app\backend\internal\config\env_loader.go
 * @LastEditTime: 2025-10-08 17:58:11
 */
package config

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/joho/godotenv"
)

var (
	envOnce     sync.Once
	envOnceLock sync.Mutex
	skipEnvLoad bool
)

// LoadEnvFiles ensures .env.local and .env files are loaded exactly once.
func LoadEnvFiles() {
	if skipEnvLoad || os.Getenv("CONFIG_SKIP_ENV_LOAD") == "1" {
		return
	}

	envOnce.Do(func() {
		// .env.local has higher priority; fall back to .env if available.
		for _, name := range []string{".env.local", ".env"} {
			if path, ok := findEnvFile(name); ok {
				if err := godotenv.Overload(path); err == nil {
					log.Printf("[config] loaded environment file: %s", path)
				}
			}
		}
	})
}

// SetEnvFileLoadingForTest toggles automatic env file loading. Intended for tests only.
func SetEnvFileLoadingForTest(enabled bool) {
	envOnceLock.Lock()
	defer envOnceLock.Unlock()

	skipEnvLoad = !enabled
	envOnce = sync.Once{}
}

func findEnvFile(name string) (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}

	dir := cwd
	for {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", false
}
