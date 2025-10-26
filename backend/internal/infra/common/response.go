/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 19:32:07
 * @FilePath: \electron-go-app\backend\internal\infra\common\response.go
 * @LastEditTime: 2025-10-09 19:32:12
 */
/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 17:02:32
 * @FilePath: \electron-go-app\backend\internal\infra\common\response.go
 * @LastEditTime: 2025-10-09 17:02:37
 */
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorCode 表示统一的错误码，便于客户端识别失败原因。
type ErrorCode string

const (
	ErrBadRequest               ErrorCode = "BAD_REQUEST"
	ErrUnauthorized             ErrorCode = "UNAUTHORIZED"
	ErrForbidden                ErrorCode = "FORBIDDEN"
	ErrNotFound                 ErrorCode = "NOT_FOUND"
	ErrConflict                 ErrorCode = "CONFLICT"
	ErrTooManyRequests          ErrorCode = "TOO_MANY_REQUESTS"
	ErrInternal                 ErrorCode = "INTERNAL_ERROR"
	ErrCaptchaInvalid           ErrorCode = "CAPTCHA_INVALID"
	ErrCaptchaExpired           ErrorCode = "CAPTCHA_EXPIRED"
	ErrCaptchaRequired          ErrorCode = "CAPTCHA_REQUIRED"
	ErrEmailNotVerified         ErrorCode = "EMAIL_NOT_VERIFIED"
	ErrEmailAlreadyVerified     ErrorCode = "EMAIL_ALREADY_VERIFIED"
	ErrVerificationTokenInvalid ErrorCode = "VERIFICATION_TOKEN_INVALID"
	ErrContentRejected          ErrorCode = "CONTENT_REJECTED"
	ErrInvalidCredentials       ErrorCode = "INVALID_CREDENTIALS"
)

// Error 描述错误响应的统一结构。
type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details any       `json:"details,omitempty"`
}

// Response 是所有接口返回的公共结构。
type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   *Error `json:"error,omitempty"`
	Meta    any    `json:"meta,omitempty"`
}

// MetaPagination 描述分页信息，可嵌入到 Response.Meta。
type MetaPagination struct {
	Page         int `json:"page"`
	PageSize     int `json:"page_size"`
	TotalItems   int `json:"total_items"`
	TotalPages   int `json:"total_pages"`
	CurrentCount int `json:"current_count"`
}

// Success 以统一格式返回成功结果。
func Success(c *gin.Context, status int, data any, meta any) {
	if status == 0 {
		status = http.StatusOK
	}

	resp := Response{
		Success: true,
		Data:    data,
	}
	if meta != nil {
		resp.Meta = meta
	}

	c.JSON(status, resp)
}

// Created 返回 201 Created 的成功响应。
func Created(c *gin.Context, data any, meta any) {
	Success(c, http.StatusCreated, data, meta)
}

// NoContent 返回 204 响应且无 body。
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Fail 以统一格式返回错误结果。
func Fail(c *gin.Context, status int, code ErrorCode, message string, details any) {
	if status == 0 {
		status = http.StatusInternalServerError
	}

	resp := Response{
		Success: false,
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}
	if details != nil {
		resp.Error.Details = details
	}

	c.JSON(status, resp)
}

// FailWithError 将系统错误映射到统一错误码。
func FailWithError(c *gin.Context, status int, err error, fallback ErrorCode) {
	if err == nil {
		Fail(c, status, fallback, http.StatusText(status), nil)
		return
	}

	code := fallback
	if code == "" {
		code = ErrInternal
	}

	Fail(c, status, code, err.Error(), nil)
}
