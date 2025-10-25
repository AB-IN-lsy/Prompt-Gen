package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type hybridStaticFS struct {
	base    http.FileSystem
	avatars http.FileSystem
}

// NewHybridStaticFS 构造可同时访问公共静态目录与头像目录的文件系统。
func NewHybridStaticFS(baseDir, avatarDir string) http.FileSystem {
	base := gin.Dir(baseDir, true)
	var avatars http.FileSystem
	if avatarDir != "" {
		avatars = gin.Dir(avatarDir, true)
	}
	return &hybridStaticFS{base: base, avatars: avatars}
}

func (fs *hybridStaticFS) Open(name string) (http.File, error) {
	clean := strings.TrimPrefix(name, "/")
	if fs.avatars != nil {
		if clean == "avatars" || strings.HasPrefix(clean, "avatars/") {
			sub := strings.TrimPrefix(clean, "avatars")
			sub = strings.TrimPrefix(sub, "/")
			if sub == "" {
				sub = "."
			}
			if file, err := fs.avatars.Open(sub); err == nil {
				return file, nil
			}
		}
	}
	return fs.base.Open(clean)
}
