package handler

import (
	"net/http"
	"strings"

	response "electron-go-app/backend/internal/infra/common"
	adminusersvc "electron-go-app/backend/internal/service/adminuser"

	"github.com/gin-gonic/gin"
)

// AdminUserHandler 负责输出管理员用户总览数据。
type AdminUserHandler struct {
	service *adminusersvc.Service
}

// NewAdminUserHandler 初始化管理员用户 Handler。
func NewAdminUserHandler(service *adminusersvc.Service) *AdminUserHandler {
	return &AdminUserHandler{service: service}
}

type adminUserListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Query    string `form:"query"`
}

// Overview 返回管理员后台的用户列表与 Prompt 概览。
func (h *AdminUserHandler) Overview(c *gin.Context) {
	if !isAdmin(c) {
		response.Fail(c, http.StatusForbidden, response.ErrForbidden, "admin privilege required", nil)
		return
	}
	if h == nil || h.service == nil {
		response.Fail(c, http.StatusServiceUnavailable, response.ErrInternal, "admin user service unavailable", nil)
		return
	}

	var req adminUserListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	result, err := h.service.ListOverview(c.Request.Context(), adminusersvc.ListParams{
		Page:     req.Page,
		PageSize: req.PageSize,
		Query:    strings.TrimSpace(req.Query),
	})
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, err.Error(), nil)
		return
	}

	response.Success(c, http.StatusOK, result, nil)
}
