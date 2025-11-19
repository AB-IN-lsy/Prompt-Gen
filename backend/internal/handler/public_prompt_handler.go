package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	promptdomain "electron-go-app/backend/internal/domain/prompt"
	response "electron-go-app/backend/internal/infra/common"
	appLogger "electron-go-app/backend/internal/infra/logger"
	"electron-go-app/backend/internal/infra/ratelimit"
	publicpromptsvc "electron-go-app/backend/internal/service/publicprompt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	// DefaultPublicSubmitLimit 控制公共库投稿默认限额。
	DefaultPublicSubmitLimit = 5
	// DefaultPublicSubmitWindow 控制公共库投稿限流窗口。
	DefaultPublicSubmitWindow = 30 * time.Minute
	// DefaultPublicDownloadLimit 控制公共库下载默认限额。
	DefaultPublicDownloadLimit = 30
	// DefaultPublicDownloadWindow 控制公共库下载限流窗口。
	DefaultPublicDownloadWindow = time.Hour
)

// PublicPromptRateLimit 描述公共库的限流配置。
type PublicPromptRateLimit struct {
	SubmitLimit    int
	SubmitWindow   time.Duration
	DownloadLimit  int
	DownloadWindow time.Duration
}

// PublicPromptHandler 负责公共 Prompt 库相关的 HTTP 接口。
type PublicPromptHandler struct {
	service        *publicpromptsvc.Service
	logger         *zap.SugaredLogger
	limiter        ratelimit.Limiter
	submitLimit    int
	submitWindow   time.Duration
	downloadLimit  int
	downloadWindow time.Duration
}

// NewPublicPromptHandler 创建公共 Prompt Handler。
func NewPublicPromptHandler(service *publicpromptsvc.Service, limiter ratelimit.Limiter, cfg PublicPromptRateLimit) *PublicPromptHandler {
	if cfg.SubmitLimit <= 0 {
		cfg.SubmitLimit = DefaultPublicSubmitLimit
	}
	if cfg.SubmitWindow <= 0 {
		cfg.SubmitWindow = DefaultPublicSubmitWindow
	}
	if cfg.DownloadLimit <= 0 {
		cfg.DownloadLimit = DefaultPublicDownloadLimit
	}
	if cfg.DownloadWindow <= 0 {
		cfg.DownloadWindow = DefaultPublicDownloadWindow
	}
	return &PublicPromptHandler{
		service:        service,
		limiter:        limiter,
		submitLimit:    cfg.SubmitLimit,
		submitWindow:   cfg.SubmitWindow,
		downloadLimit:  cfg.DownloadLimit,
		downloadWindow: cfg.DownloadWindow,
	}
}

// ensureLogger 确保 handler 拥有基础日志实例。
func (h *PublicPromptHandler) ensureLogger() *zap.SugaredLogger {
	if h.logger == nil {
		h.logger = appLogger.S().With("component", "public_prompt.handler")
	}
	return h.logger
}

// scope 返回带操作名的日志实例。
func (h *PublicPromptHandler) scope(operation string) *zap.SugaredLogger {
	return h.ensureLogger().With("operation", operation)
}

// allow 执行限流判断。
func (h *PublicPromptHandler) allow(c *gin.Context, key string, limit int, window time.Duration) bool {
	if h.limiter == nil || limit <= 0 {
		return true
	}
	res, err := h.limiter.Allow(c.Request.Context(), key, limit, window)
	if err != nil {
		h.ensureLogger().Warnw("public prompt ratelimit failure", "key", key, "error", err)
		return true
	}
	if res.Allowed {
		return true
	}
	retry := int(res.RetryAfter.Seconds())
	response.Fail(c, http.StatusTooManyRequests, response.ErrTooManyRequests, "请求过于频繁，请稍后再试", gin.H{"retry_after_seconds": retry})
	return false
}

// List 公共库列表查询，普通用户仅能看到已审核通过的条目。
func (h *PublicPromptHandler) List(c *gin.Context) {
	log := h.scope("list")
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}

	defaultPageSize, maxPageSize := h.service.ListPageSizeBounds()
	if defaultPageSize <= 0 {
		defaultPageSize = publicpromptsvc.DefaultListPageSize
	}
	if maxPageSize <= 0 {
		maxPageSize = publicpromptsvc.DefaultListMaxPageSize
	}

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page <= 0 {
		page = 1
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(defaultPageSize)))
	if err != nil || pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if maxPageSize > 0 && pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	sortByParam := strings.ToLower(strings.TrimSpace(c.DefaultQuery("sort_by", "score")))
	allowedSorts := map[string]bool{
		"score":      true,
		"downloads":  true,
		"likes":      true,
		"visits":     true,
		"updated_at": true,
		"created_at": true,
		"":           true,
	}
	if !allowedSorts[sortByParam] {
		sortByParam = ""
	}
	sortOrderParam := strings.ToLower(strings.TrimSpace(c.DefaultQuery("sort_order", "desc")))
	sortOrder := "DESC"
	if sortOrderParam == "asc" {
		sortOrder = "ASC"
	}

	filter := publicpromptsvc.ListFilter{
		Query:        c.Query("q"),
		Page:         page,
		PageSize:     pageSize,
		ViewerUserID: userID,
		SortBy:       sortByParam,
		SortOrder:    sortOrder,
	}
	statusParam := strings.TrimSpace(c.Query("status"))
	authorParam := strings.TrimSpace(c.Query("author_id"))
	var requestedAuthorID uint
	if authorParam != "" {
		if authorID, convErr := strconv.Atoi(authorParam); convErr == nil && authorID > 0 {
			requestedAuthorID = uint(authorID)
		}
	}
	if isAdmin(c) {
		if statusParam != "" && statusParam != "all" {
			filter.Status = statusParam
		}
		if requestedAuthorID > 0 {
			filter.AuthorUserID = requestedAuthorID
		}
	} else {
		switch statusParam {
		case "", "all", promptdomain.PublicPromptStatusApproved:
			filter.OnlyApproved = true
		case promptdomain.PublicPromptStatusPending, promptdomain.PublicPromptStatusRejected:
			filter.Status = statusParam
			filter.AuthorUserID = userID
		default:
			filter.OnlyApproved = true
		}
		if requestedAuthorID > 0 {
			filter.AuthorUserID = requestedAuthorID
			filter.Status = ""
			filter.OnlyApproved = true
		}
	}

	result, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		log.Errorw("list public prompts failed", "error", err)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "获取公共 Prompt 列表失败", nil)
		return
	}

	items := make([]gin.H, 0, len(result.Items))
	for _, item := range result.Items {
		reviewReason := ""
		if isAdmin(c) || item.AuthorUserID == userID {
			reviewReason = item.ReviewReason
		}
		items = append(items, gin.H{
			"id":             item.ID,
			"title":          item.Title,
			"topic":          item.Topic,
			"summary":        item.Summary,
			"model":          item.Model,
			"status":         item.Status,
			"download_count": item.DownloadCount,
			"language":       item.Language,
			"tags":           item.Tags,
			"created_at":     item.CreatedAt,
			"updated_at":     item.UpdatedAt,
			"review_reason":  reviewReason,
			"author_user_id": item.AuthorUserID,
			"author":         publicPromptAuthorPayload(item.Author),
			"reviewer_user_id": func() *uint {
				if item.ReviewerUserID == nil {
					return nil
				}
				id := *item.ReviewerUserID
				return &id
			}(),
			"like_count":    item.LikeCount,
			"is_liked":      item.IsLiked,
			"visit_count":   item.VisitCount,
			"quality_score": item.QualityScore,
		})
	}

	response.Success(
		c,
		http.StatusOK,
		gin.H{"items": items},
		response.MetaPagination{
			Page:         result.Page,
			PageSize:     result.PageSize,
			TotalItems:   int(result.Total),
			TotalPages:   result.TotalPages,
			CurrentCount: len(items),
		},
	)
}

// Get 返回公共 Prompt 详情。
func (h *PublicPromptHandler) Get(c *gin.Context) {
	log := h.scope("detail")
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}
	idVal := strings.TrimSpace(c.Param("id"))
	idNum, err := strconv.Atoi(idVal)
	if err != nil || idNum <= 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "invalid id", nil)
		return
	}

	entity, err := h.service.Get(c.Request.Context(), uint(idNum), userID)
	if err != nil {
		if errors.Is(err, publicpromptsvc.ErrPublicPromptNotFound) {
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, "公共 Prompt 不存在", nil)
			return
		}
		log.Errorw("get public prompt failed", "error", err, "id", idNum)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "获取公共 Prompt 详情失败", nil)
		return
	}
	if entity.Status != promptdomain.PublicPromptStatusApproved {
		if !isAdmin(c) && entity.AuthorUserID != userID {
			response.Fail(c, http.StatusForbidden, response.ErrForbidden, "公共 Prompt 尚未通过审核", nil)
			return
		}
	}
	reviewReason := ""
	if isAdmin(c) || entity.AuthorUserID == userID {
		reviewReason = entity.ReviewReason
	}
	response.Success(c, http.StatusOK, gin.H{
		"id":                entity.ID,
		"title":             entity.Title,
		"topic":             entity.Topic,
		"source_prompt_id":  entity.SourcePromptID,
		"summary":           entity.Summary,
		"body":              entity.Body,
		"instructions":      entity.Instructions,
		"positive_keywords": entity.PositiveKeywords,
		"negative_keywords": entity.NegativeKeywords,
		"tags":              entity.Tags,
		"model":             entity.Model,
		"language":          entity.Language,
		"status":            entity.Status,
		"download_count":    entity.DownloadCount,
		"created_at":        entity.CreatedAt,
		"updated_at":        entity.UpdatedAt,
		"review_reason":     reviewReason,
		"author_user_id":    entity.AuthorUserID,
		"author":            publicPromptAuthorPayload(entity.Author),
		"like_count":        entity.LikeCount,
		"is_liked":          entity.IsLiked,
		"visit_count":       entity.VisitCount,
		"quality_score":     entity.QualityScore,
	}, nil)
}

// SubmitRequest 绑定公共 Prompt 提交参数。
type SubmitRequest struct {
	SourcePromptID   *uint  `json:"source_prompt_id"`
	Title            string `json:"title" binding:"required"`
	Topic            string `json:"topic" binding:"required"`
	Summary          string `json:"summary" binding:"required"`
	Body             string `json:"body" binding:"required"`
	Instructions     string `json:"instructions" binding:"required"`
	PositiveKeywords string `json:"positive_keywords" binding:"required"`
	NegativeKeywords string `json:"negative_keywords" binding:"required"`
	Tags             string `json:"tags" binding:"required"`
	Model            string `json:"model" binding:"required"`
	Language         string `json:"language"`
}

// Submit 提交公共 Prompt 供管理员审核。
func (h *PublicPromptHandler) Submit(c *gin.Context) {
	log := h.scope("submit")
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}
	if !h.service.AllowSubmission() {
		response.Fail(c, http.StatusForbidden, response.ErrForbidden, "公共库投稿在当前模式已关闭", nil)
		return
	}

	if !h.allow(c, fmt.Sprintf("public:submit:%d", userID), h.submitLimit, h.submitWindow) {
		return
	}

	var req SubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	entity, err := h.service.Submit(c.Request.Context(), publicpromptsvc.SubmitInput{
		AuthorUserID:     userID,
		SourcePromptID:   req.SourcePromptID,
		Title:            req.Title,
		Topic:            req.Topic,
		Summary:          req.Summary,
		Body:             req.Body,
		Instructions:     req.Instructions,
		PositiveKeywords: req.PositiveKeywords,
		NegativeKeywords: req.NegativeKeywords,
		Tags:             req.Tags,
		Model:            req.Model,
		Language:         req.Language,
	})
	if err != nil {
		if errors.Is(err, publicpromptsvc.ErrSubmissionDisabled) {
			response.Fail(c, http.StatusForbidden, response.ErrForbidden, "公共库投稿在当前模式已关闭", nil)
			return
		}
		log.Errorw("submit public prompt failed", "error", err, "user_id", userID)
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	response.Created(c, gin.H{
		"id":      entity.ID,
		"status":  entity.Status,
		"created": entity.CreatedAt,
	}, nil)
}

// ReviewRequest 描述审核请求体。
type ReviewRequest struct {
	Status string `json:"status" binding:"required"`
	Reason string `json:"reason"`
}

// Review 管理员审核公共 Prompt。
func (h *PublicPromptHandler) Review(c *gin.Context) {
	log := h.scope("review")
	reviewerID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}
	if !isAdmin(c) {
		response.Fail(c, http.StatusForbidden, response.ErrForbidden, "仅管理员可执行审核", nil)
		return
	}

	idVal := strings.TrimSpace(c.Param("id"))
	idNum, err := strconv.Atoi(idVal)
	if err != nil || idNum <= 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "invalid id", nil)
		return
	}

	var req ReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}

	if err := h.service.Review(c.Request.Context(), publicpromptsvc.ReviewInput{
		ReviewerUserID: reviewerID,
		PromptID:       uint(idNum),
		Status:         req.Status,
		Reason:         req.Reason,
	}); err != nil {
		switch {
		case errors.Is(err, publicpromptsvc.ErrPublicPromptNotFound):
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, "公共 Prompt 不存在", nil)
		case errors.Is(err, publicpromptsvc.ErrReviewStatusInvalid):
			response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "状态非法，只能是 approved 或 rejected", nil)
		default:
			log.Errorw("review public prompt failed", "error", err, "prompt_id", idNum, "reviewer", reviewerID)
			response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "审核公共 Prompt 失败", nil)
		}
		return
	}

	response.NoContent(c)
}

// Download 公开库下载，将条目复制到用户个人库。
func (h *PublicPromptHandler) Download(c *gin.Context) {
	log := h.scope("download")
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}
	idVal := strings.TrimSpace(c.Param("id"))
	idNum, err := strconv.Atoi(idVal)
	if err != nil || idNum <= 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "invalid id", nil)
		return
	}

	if !h.allow(c, fmt.Sprintf("public:download:%d", userID), h.downloadLimit, h.downloadWindow) {
		return
	}

	prompt, err := h.service.Download(c.Request.Context(), publicpromptsvc.DownloadInput{
		UserID:         userID,
		PublicPromptID: uint(idNum),
	})
	if err != nil {
		switch {
		case errors.Is(err, publicpromptsvc.ErrPublicPromptNotFound):
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, "公共 Prompt 不存在", nil)
		case errors.Is(err, publicpromptsvc.ErrPromptNotApproved):
			response.Fail(c, http.StatusForbidden, response.ErrForbidden, "公共 Prompt 尚未通过审核", nil)
		default:
			log.Errorw("download public prompt failed", "error", err, "user_id", userID, "prompt_id", idNum)
			response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "下载公共 Prompt 失败", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"prompt_id": prompt.ID,
		"status":    prompt.Status,
	}, nil)
}

// Like 为公共 Prompt 点赞。
func (h *PublicPromptHandler) Like(c *gin.Context) {
	h.handleLike(c, true)
}

// Unlike 取消公共 Prompt 点赞。
func (h *PublicPromptHandler) Unlike(c *gin.Context) {
	h.handleLike(c, false)
}

func (h *PublicPromptHandler) handleLike(c *gin.Context, like bool) {
	action := "like"
	if !like {
		action = "unlike"
	}
	log := h.scope(action)
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}
	idVal := strings.TrimSpace(c.Param("id"))
	idNum, err := strconv.Atoi(idVal)
	if err != nil || idNum <= 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "invalid id", nil)
		return
	}

	var result publicpromptsvc.LikeResult
	if like {
		result, err = h.service.Like(c.Request.Context(), userID, uint(idNum))
	} else {
		result, err = h.service.Unlike(c.Request.Context(), userID, uint(idNum))
	}
	if err != nil {
		switch {
		case errors.Is(err, publicpromptsvc.ErrPublicPromptNotFound):
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, "公共 Prompt 不存在", nil)
		case errors.Is(err, publicpromptsvc.ErrLikeNotAvailable):
			response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "当前公共 Prompt 不支持点赞", nil)
		default:
			log.Errorw("toggle public prompt like failed", "error", err, "prompt_id", idNum, "user_id", userID, "like", like)
			response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "更新点赞状态失败", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"liked":      result.Liked,
		"like_count": result.LikeCount,
	}, nil)
}

// Delete 删除指定公共 Prompt，需管理员权限。
func (h *PublicPromptHandler) Delete(c *gin.Context) {
	log := h.scope("delete")
	if !isAdmin(c) {
		response.Fail(c, http.StatusForbidden, response.ErrForbidden, "仅管理员可操作", nil)
		return
	}
	if _, ok := extractUserID(c); !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "missing user id", nil)
		return
	}
	idVal := strings.TrimSpace(c.Param("id"))
	idNum, err := strconv.Atoi(idVal)
	if err != nil || idNum <= 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "invalid id", nil)
		return
	}

	if err := h.service.Delete(c.Request.Context(), uint(idNum)); err != nil {
		if errors.Is(err, publicpromptsvc.ErrPublicPromptNotFound) {
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, "公共 Prompt 不存在", nil)
			return
		}
		log.Errorw("delete public prompt failed", "error", err, "id", idNum)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "删除公共 Prompt 失败", nil)
		return
	}

	response.NoContent(c)
}

// publicPromptAuthorPayload 将创作者信息转换为响应体，避免泄露敏感字段。
func publicPromptAuthorPayload(brief *promptdomain.UserBrief) gin.H {
	if brief == nil {
		return nil
	}
	return gin.H{
		"id":         brief.ID,
		"username":   brief.Username,
		"avatar_url": brief.AvatarURL,
		"headline":   brief.Headline,
		"bio":        brief.Bio,
		"location":   brief.Location,
		"website":    brief.Website,
		"banner_url": brief.BannerURL,
	}
}
