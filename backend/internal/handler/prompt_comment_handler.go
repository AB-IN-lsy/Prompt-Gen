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
	"electron-go-app/backend/internal/infra/ratelimit"
	promptcommentsvc "electron-go-app/backend/internal/service/promptcomment"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PromptCommentRateLimit 控制评论创建频率。
type PromptCommentRateLimit struct {
	CreateLimit  int
	CreateWindow time.Duration
}

// PromptCommentHandler 负责 Prompt 评论相关的 HTTP 接口。
type PromptCommentHandler struct {
	service      *promptcommentsvc.Service
	limiter      ratelimit.Limiter
	createLimit  int
	createWindow time.Duration
	logger       *zap.SugaredLogger
}

// NewPromptCommentHandler 创建评论 Handler。
func NewPromptCommentHandler(service *promptcommentsvc.Service, limiter ratelimit.Limiter, cfg PromptCommentRateLimit) *PromptCommentHandler {
	if cfg.CreateLimit < 0 {
		cfg.CreateLimit = 0
	}
	if cfg.CreateWindow < 0 {
		cfg.CreateWindow = 0
	}
	return &PromptCommentHandler{
		service:      service,
		limiter:      limiter,
		createLimit:  cfg.CreateLimit,
		createWindow: cfg.CreateWindow,
	}
}

// ensureLogger 确保内部使用的日志记录器已初始化。
func (h *PromptCommentHandler) ensureLogger() *zap.SugaredLogger {
	if h.logger == nil {
		h.logger = zap.NewNop().Sugar()
	}
	return h.logger
}

// scope 为当前操作构造带上下文的日志实例。
func (h *PromptCommentHandler) scope(operation string) *zap.SugaredLogger {
	return h.ensureLogger().With("component", "prompt_comment.handler", "operation", operation)
}

// allow 结合限流器检查用户是否可以继续执行操作。
func (h *PromptCommentHandler) allow(c *gin.Context, key string, limit int, window time.Duration) bool {
	if h.limiter == nil || limit <= 0 {
		return true
	}
	res, err := h.limiter.Allow(c.Request.Context(), key, limit, window)
	if err != nil {
		h.ensureLogger().Warnw("comment rate limiter failed", "error", err, "key", key)
		return true
	}
	if res.Allowed {
		return true
	}
	retry := int(res.RetryAfter.Seconds())
	response.Fail(c, http.StatusTooManyRequests, response.ErrTooManyRequests, "评论过于频繁，请稍后再试", gin.H{
		"retry_after_seconds": retry,
	})
	return false
}

// List 返回目标 Prompt 的评论列表。
func (h *PromptCommentHandler) List(c *gin.Context) {
	log := h.scope("list")
	idVal := strings.TrimSpace(c.Param("id"))
	idNum, err := strconv.Atoi(idVal)
	if err != nil || idNum <= 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "无效的 Prompt 编号", nil)
		return
	}
	promptID := uint(idNum)
	viewerID, _ := extractUserID(c)
	status := promptdomain.PromptCommentStatusApproved
	if isAdmin(c) {
		statusParam := strings.TrimSpace(c.DefaultQuery("status", ""))
		if strings.EqualFold(statusParam, "all") || statusParam == "" {
			status = ""
		} else {
			status = statusParam
		}
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page <= 0 {
		page = 1
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "0"))
	if err != nil {
		pageSize = 0
	}
	result, err := h.service.List(c.Request.Context(), promptcommentsvc.ListCommentsInput{
		PromptID: promptID,
		Status:   status,
		Page:     page,
		PageSize: pageSize,
		ViewerID: viewerID,
	})
	if err != nil {
		log.Errorw("list comments failed", "error", err, "prompt_id", promptID)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "获取评论列表失败", nil)
		return
	}
	exposeReview := isAdmin(c)
	items := make([]gin.H, 0, len(result.Items))
	for _, thread := range result.Items {
		items = append(items, h.buildThreadResponse(thread, exposeReview))
	}
	meta := response.MetaPagination{
		Page:         result.Page,
		PageSize:     result.PageSize,
		TotalItems:   int(result.Total),
		TotalPages:   result.TotalPages,
		CurrentCount: len(result.Items),
	}
	response.Success(c, http.StatusOK, gin.H{
		"items": items,
	}, meta)
}

type createCommentRequest struct {
	Body     string `json:"body" binding:"required"`
	ParentID *uint  `json:"parent_id"`
}

// Create 发表新的评论。
func (h *PromptCommentHandler) Create(c *gin.Context) {
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "缺少用户信息", nil)
		return
	}
	idVal := strings.TrimSpace(c.Param("id"))
	idNum, err := strconv.Atoi(idVal)
	if err != nil || idNum <= 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "无效的 Prompt 编号", nil)
		return
	}
	if !h.allow(c, fmt.Sprintf("prompt:comment:create:%d", userID), h.createLimit, h.createWindow) {
		return
	}
	var req createCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}
	comment, err := h.service.Create(c.Request.Context(), promptcommentsvc.CreateCommentInput{
		PromptID: uint(idNum),
		UserID:   userID,
		ParentID: req.ParentID,
		Body:     req.Body,
	})
	if err != nil {
		switch {
		case errors.Is(err, promptcommentsvc.ErrPromptNotFound):
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, "目标 Prompt 不存在", nil)
			return
		case errors.Is(err, promptcommentsvc.ErrParentCommentInvalid):
			response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "回复目标无效", nil)
			return
		default:
			response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
			return
		}
	}
	response.Created(c, h.buildCommentResponse(*comment, isAdmin(c)), nil)
}

type reviewCommentRequest struct {
	Status string `json:"status" binding:"required"`
	Note   string `json:"note"`
}

// Review 审核或驳回评论。
func (h *PromptCommentHandler) Review(c *gin.Context) {
	if !isAdmin(c) {
		response.Fail(c, http.StatusForbidden, response.ErrForbidden, "仅管理员可审核评论", nil)
		return
	}
	reviewer, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "缺少用户信息", nil)
		return
	}
	idVal := strings.TrimSpace(c.Param("id"))
	idNum, err := strconv.Atoi(idVal)
	if err != nil || idNum <= 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "无效的评论编号", nil)
		return
	}
	var req reviewCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		return
	}
	comment, err := h.service.Review(c.Request.Context(), promptcommentsvc.ReviewInput{
		CommentID: uint(idNum),
		Reviewer:  reviewer,
		Status:    req.Status,
		Note:      req.Note,
	})
	if err != nil {
		switch {
		case errors.Is(err, promptcommentsvc.ErrCommentNotFound):
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, "评论不存在", nil)
			return
		default:
			response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
			return
		}
	}
	response.Success(c, http.StatusOK, h.buildCommentResponse(*comment, true), nil)
}

// Delete 移除指定评论，管理员可级联删除整条回复链。
func (h *PromptCommentHandler) Delete(c *gin.Context) {
	if !isAdmin(c) {
		response.Fail(c, http.StatusForbidden, response.ErrForbidden, "仅管理员可删除评论", nil)
		return
	}
	idVal := strings.TrimSpace(c.Param("id"))
	commentID, err := strconv.Atoi(idVal)
	if err != nil || commentID <= 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "无效的评论编号", nil)
		return
	}
	if err := h.service.Delete(c.Request.Context(), uint(commentID)); err != nil {
		if errors.Is(err, promptcommentsvc.ErrCommentNotFound) {
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, "评论不存在或已删除", nil)
			return
		}
		h.scope("delete").Errorw("delete comment failed", "error", err, "comment_id", commentID)
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "删除评论失败", nil)
		return
	}
	response.Success(c, http.StatusOK, gin.H{
		"id": commentID,
	}, nil)
}

// Like 为评论点赞。
func (h *PromptCommentHandler) Like(c *gin.Context) {
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "缺少用户信息", nil)
		return
	}
	idVal := strings.TrimSpace(c.Param("id"))
	idNum, err := strconv.Atoi(idVal)
	if err != nil || idNum <= 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "无效的评论编号", nil)
		return
	}
	result, err := h.service.LikeComment(c.Request.Context(), promptcommentsvc.UpdateCommentLikeInput{
		CommentID: uint(idNum),
		UserID:    userID,
	})
	if err != nil {
		switch {
		case errors.Is(err, promptcommentsvc.ErrCommentNotFound):
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, "评论不存在或已删除", nil)
		case errors.Is(err, promptcommentsvc.ErrCommentNotApproved):
			response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		default:
			h.scope("like").Errorw("like comment failed", "error", err, "comment_id", idNum, "user_id", userID)
			response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "点赞评论失败", nil)
		}
		return
	}
	response.Success(c, http.StatusOK, gin.H{
		"like_count": result.LikeCount,
		"liked":      result.Liked,
	}, nil)
}

// Unlike 取消对评论的点赞。
func (h *PromptCommentHandler) Unlike(c *gin.Context) {
	userID, ok := extractUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.ErrUnauthorized, "缺少用户信息", nil)
		return
	}
	idVal := strings.TrimSpace(c.Param("id"))
	idNum, err := strconv.Atoi(idVal)
	if err != nil || idNum <= 0 {
		response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, "无效的评论编号", nil)
		return
	}
	result, err := h.service.UnlikeComment(c.Request.Context(), promptcommentsvc.UpdateCommentLikeInput{
		CommentID: uint(idNum),
		UserID:    userID,
	})
	if err != nil {
		switch {
		case errors.Is(err, promptcommentsvc.ErrCommentNotFound):
			response.Fail(c, http.StatusNotFound, response.ErrNotFound, "评论不存在或已删除", nil)
		case errors.Is(err, promptcommentsvc.ErrCommentNotApproved):
			response.Fail(c, http.StatusBadRequest, response.ErrBadRequest, err.Error(), nil)
		default:
			h.scope("unlike").Errorw("unlike comment failed", "error", err, "comment_id", idNum, "user_id", userID)
			response.Fail(c, http.StatusInternalServerError, response.ErrInternal, "取消点赞失败", nil)
		}
		return
	}
	response.Success(c, http.StatusOK, gin.H{
		"like_count": result.LikeCount,
		"liked":      result.Liked,
	}, nil)
}

// buildThreadResponse 将评论线程展开为前端可消费的 JSON 结构。
func (h *PromptCommentHandler) buildThreadResponse(thread promptcommentsvc.CommentThread, includeReview bool) gin.H {
	replies := make([]gin.H, 0, len(thread.Replies))
	for _, reply := range thread.Replies {
		replies = append(replies, h.buildCommentResponse(reply, includeReview))
	}
	root := h.buildCommentResponse(thread.Root, includeReview)
	root["replies"] = replies
	return root
}

// buildCommentResponse 转换单条评论的字段，按需附带审核信息。
func (h *PromptCommentHandler) buildCommentResponse(comment promptdomain.PromptComment, includeReview bool) gin.H {
	var author gin.H
	if comment.Author != nil {
		author = gin.H{
			"id":         comment.Author.ID,
			"username":   comment.Author.Username,
			"email":      comment.Author.Email,
			"avatar_url": comment.Author.AvatarURL,
		}
	} else {
		author = nil
	}
	resp := gin.H{
		"id":          comment.ID,
		"prompt_id":   comment.PromptID,
		"user_id":     comment.UserID,
		"parent_id":   comment.ParentID,
		"root_id":     comment.RootID,
		"body":        comment.Body,
		"status":      comment.Status,
		"like_count":  comment.LikeCount,
		"is_liked":    comment.IsLiked,
		"reply_count": comment.ReplyCount,
		"author":      author,
		"created_at":  comment.CreatedAt,
		"updated_at":  comment.UpdatedAt,
	}
	if includeReview {
		resp["review_note"] = comment.ReviewNote
		resp["reviewer_user_id"] = comment.ReviewerUserID
	}
	return resp
}
