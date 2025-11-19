package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	promptdomain "electron-go-app/backend/internal/domain/prompt"
	response "electron-go-app/backend/internal/infra/common"
	appLogger "electron-go-app/backend/internal/infra/logger"
	publicpromptsvc "electron-go-app/backend/internal/service/publicprompt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CreatorHandler 负责创作者主页相关接口。
type CreatorHandler struct {
	service *publicpromptsvc.Service
	logger  *zap.SugaredLogger
}

// NewCreatorHandler 构造创作者 Handler。
func NewCreatorHandler(service *publicpromptsvc.Service) *CreatorHandler {
	return &CreatorHandler{
		service: service,
		logger:  appLogger.S().With("component", "creator.handler"),
	}
}

// GetProfile 返回指定创作者的公开资料与精选 Prompt。
func (h *CreatorHandler) GetProfile(c *gin.Context) {
	log := h.logger.With("operation", "profile")

	viewerID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}

	creatorParam := strings.TrimSpace(c.Param("id"))
	creatorID, err := strconv.Atoi(creatorParam)
	if err != nil || creatorID <= 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "invalid creator id", nil)
		return
	}

	profile, err := h.service.AuthorProfile(c.Request.Context(), uint(creatorID), viewerID)
	if err != nil {
		if errors.Is(err, publicpromptsvc.ErrAuthorNotFound) {
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, "创作者不存在", nil)
			return
		}
		log.Errorw("load creator profile failed", "error", err, "creator_id", creatorID)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "获取创作者信息失败", nil)
		return
	}

	recent := make([]gin.H, 0, len(profile.RecentPrompts))
	for _, item := range profile.RecentPrompts {
		recent = append(recent, buildCreatorPromptPayload(item))
	}

	payload := gin.H{
		"creator": publicPromptAuthorPayload(profile.Author),
		"stats": gin.H{
			"prompt_count":    profile.Stats.PromptCount,
			"total_downloads": profile.Stats.TotalDownloads,
			"total_likes":     profile.Stats.TotalLikes,
			"total_visits":    profile.Stats.TotalVisits,
		},
		"recent_prompts": recent,
	}

	response.Success(c, http.StatusOK, payload, nil)
}

// buildCreatorPromptPayload 将公共 Prompt 转换为创作者主页的精选卡片结构。
func buildCreatorPromptPayload(item promptdomain.PublicPrompt) gin.H {
	return gin.H{
		"id":             item.ID,
		"title":          item.Title,
		"topic":          item.Topic,
		"summary":        item.Summary,
		"model":          item.Model,
		"language":       item.Language,
		"status":         item.Status,
		"tags":           item.Tags,
		"download_count": item.DownloadCount,
		"visit_count":    item.VisitCount,
		"like_count":     item.LikeCount,
		"quality_score":  item.QualityScore,
		"author":         publicPromptAuthorPayload(item.Author),
		"updated_at":     item.UpdatedAt,
		"created_at":     item.CreatedAt,
	}
}
