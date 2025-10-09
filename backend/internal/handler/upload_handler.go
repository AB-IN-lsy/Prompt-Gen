/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 02:33:03
 * @FilePath: \electron-go-app\backend\internal\handler\upload_handler.go
 * @LastEditTime: 2025-10-10 02:33:09
 */
package handler

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	response "electron-go-app/backend/internal/infra/common"
	appLogger "electron-go-app/backend/internal/infra/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// UploadHandler 负责处理文件上传请求（目前仅支持用户头像）。
//
// 它会将文件写入指定的 storageRoot 目录，并返回可通过静态路由访问的相对路径。
type UploadHandler struct {
	storageRoot string
	logger      *zap.SugaredLogger
}

// NewUploadHandler 构造上传 handler，storageRoot 指向静态资源目录，例如 public/avatars。
func NewUploadHandler(storageRoot string) *UploadHandler {
	baseLogger := appLogger.S().With("component", "upload.handler")
	return &UploadHandler{storageRoot: storageRoot, logger: baseLogger}
}

// UploadAvatar 处理头像上传：
//  1. 校验文件是否存在、大小是否符合限制（<=5MB）、格式是否为常见图片类型；
//  2. 为文件生成 UUID+扩展名，写入 storageRoot 目录；
//  3. 返回可被前端直接引用的 `/static/avatars/<filename>` 路径。
//
// 该接口需要通过鉴权中间件保护，防止匿名用户滥用上传能力。
func (h *UploadHandler) UploadAvatar(c *gin.Context) {
	log := h.scope("upload_avatar")

	file, err := c.FormFile("avatar")
	if err != nil {
		log.Warnw("missing avatar file", "error", err)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "avatar file is required", nil)
		return
	}

	if file.Size == 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "avatar file is empty", nil)
		return
	}

	if file.Size > 5*1024*1024 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "avatar file is too large", nil)
		return
	}

	if err := h.ensureStorageDir(); err != nil {
		log.Errorw("ensure storage dir failed", "error", err)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "failed to prepare storage", nil)
		return
	}

	if !isSupportedImage(file) {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "unsupported avatar format", nil)
		return
	}

	filename := h.generateFilename(file.Filename)
	dst := filepath.Join(h.storageRoot, filename)

	if err := c.SaveUploadedFile(file, dst); err != nil {
		log.Errorw("save avatar failed", "error", err)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "failed to save avatar", nil)
		return
	}

	avatarURL := fmt.Sprintf("/static/avatars/%s", filename)

	response.Success(c, http.StatusCreated, gin.H{
		"avatar_url": avatarURL,
	}, nil)
}

func (h *UploadHandler) ensureStorageDir() error {
	return os.MkdirAll(h.storageRoot, 0o755)
}

// generateFilename 使用 UUID 与原始扩展名生成存储文件名，避免重复覆盖。
func (h *UploadHandler) generateFilename(original string) string {
	ext := strings.ToLower(filepath.Ext(original))
	if ext == "" {
		ext = ".png"
	}
	return fmt.Sprintf("%s%s", uuid.NewString(), ext)
}

// isSupportedImage 根据 Content-Type 判断文件是否为允许的图片格式。
func isSupportedImage(fileHeader *multipart.FileHeader) bool {
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		return false
	}
	switch {
	case strings.HasPrefix(contentType, "image/jpeg"):
		return true
	case strings.HasPrefix(contentType, "image/png"):
		return true
	case strings.HasPrefix(contentType, "image/gif"):
		return true
	case strings.HasPrefix(contentType, "image/webp"):
		return true
	default:
		return false
	}
}

func (h *UploadHandler) ensureLogger() *zap.SugaredLogger {
	if h.logger == nil {
		h.logger = appLogger.S().With("component", "upload.handler")
	}
	return h.logger
}

func (h *UploadHandler) scope(operation string) *zap.SugaredLogger {
	return h.ensureLogger().With("operation", operation, "timestamp", time.Now().Format(time.RFC3339Nano))
}
