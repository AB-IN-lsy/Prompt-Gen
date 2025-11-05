package handler

import (
	"net/http"

	response "electron-go-app/backend/internal/infra/common"
	adminmetricssvc "electron-go-app/backend/internal/service/adminmetrics"

	"github.com/gin-gonic/gin"
)

// AdminMetricsHandler 负责输出管理员仪表盘所需的缓存指标。
type AdminMetricsHandler struct {
	service *adminmetricssvc.Service
}

// NewAdminMetricsHandler 构造指标 Handler，注入后台缓存服务。
func NewAdminMetricsHandler(service *adminmetricssvc.Service) *AdminMetricsHandler {
	return &AdminMetricsHandler{service: service}
}

// Overview 返回最近数日的指标快照，仅允许管理员访问。
func (h *AdminMetricsHandler) Overview(c *gin.Context) {
	if !isAdmin(c) {
		response.Fail(c, http.StatusForbidden, response.ErrForbidden, "admin privilege required", nil)
		return
	}
	if h == nil || h.service == nil {
		response.Fail(c, http.StatusServiceUnavailable, response.ErrInternal, "admin metrics unavailable", nil)
		return
	}
	response.Success(c, http.StatusOK, h.service.Snapshot(), nil)
}
