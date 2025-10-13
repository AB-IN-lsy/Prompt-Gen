package handler

import (
	"net/http"
	"strings"

	response "electron-go-app/backend/internal/infra/common"
	appLogger "electron-go-app/backend/internal/infra/logger"
	"electron-go-app/backend/internal/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// IPGuardHandler 提供黑名单可视化接口，仅允许管理员访问。
type IPGuardHandler struct {
	guard  *middleware.IPGuardMiddleware
	logger *zap.SugaredLogger
}

// NewIPGuardHandler 构建 Handler，并复用统一日志实例。
func NewIPGuardHandler(guard *middleware.IPGuardMiddleware) *IPGuardHandler {
	baseLogger := appLogger.S().With("component", "handler.ipguard")
	return &IPGuardHandler{guard: guard, logger: baseLogger}
}

// ListBans 返回当前仍在 Redis 中的封禁 IP 列表。
func (h *IPGuardHandler) ListBans(c *gin.Context) {
	if !isAdmin(c) {
		response.Fail(c, http.StatusForbidden, response.ErrForbidden, "admin privilege required", nil)
		return
	}
	if h.guard == nil {
		response.Fail(c, http.StatusServiceUnavailable, response.ErrInternal, "ip guard disabled", nil)
		return
	}

	entries, err := h.guard.ListBlacklistEntries(c.Request.Context())
	if err != nil {
		h.logger.Errorw("list blacklist failed", "error", err)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "list blacklist failed", nil)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"items": entries}, nil)
}

// RemoveBan 手动解除给定 IP 在 Redis 中的封禁。
func (h *IPGuardHandler) RemoveBan(c *gin.Context) {
	if !isAdmin(c) {
		response.Fail(c, http.StatusForbidden, response.ErrForbidden, "admin privilege required", nil)
		return
	}
	if h.guard == nil {
		response.Fail(c, http.StatusServiceUnavailable, response.ErrInternal, "ip guard disabled", nil)
		return
	}

	ip := strings.TrimSpace(c.Param("ip"))
	if ip == "" {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "ip required", nil)
		return
	}

	if err := h.guard.RemoveFromBlacklist(c.Request.Context(), ip); err != nil {
		h.logger.Errorw("remove blacklist failed", "ip", ip, "error", err)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "remove blacklist failed", nil)
		return
	}

	response.NoContent(c)
}
