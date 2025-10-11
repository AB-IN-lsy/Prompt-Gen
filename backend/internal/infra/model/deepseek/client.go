package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// defaultBaseURL 为 DeepSeek Chat Completion API 的默认入口地址。
	defaultBaseURL = "https://api.deepseek.com/v1"
	// defaultTimeout 控制 HTTP 请求的默认超时时间。
	defaultTimeout = 30 * time.Second
)

// Client 封装与 DeepSeek 服务的 HTTP 交互。
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Option 用于自定义 Client 行为。
type Option func(*Client)

// WithBaseURL 设置 DeepSeek API 的自定义基础地址。
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithHTTPClient 允许传入调用方自定义的 http.Client。
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// NewClient 构造 DeepSeek 客户端，默认使用 30 秒超时。
func NewClient(apiKey string, opts ...Option) *Client {
	client := &Client{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
	for _, opt := range opts {
		opt(client)
	}
	if client.httpClient == nil {
		client.httpClient = &http.Client{Timeout: defaultTimeout}
	}
	if client.baseURL == "" {
		client.baseURL = defaultBaseURL
	}
	return client
}

// endpoint 拼接最终请求路径。
func (c *Client) endpoint(path string) string {
	return c.baseURL + path
}

// APIError 封装 DeepSeek 返回的错误响应，便于上层识别。
type APIError struct {
	StatusCode int             `json:"-"`
	Type       string          `json:"type,omitempty"`
	Code       string          `json:"code,omitempty"`
	Message    string          `json:"message"`
	Raw        json.RawMessage `json:"raw,omitempty"`
}

// Error 实现 error 接口。
func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	desc := e.Message
	if e.Code != "" {
		desc = fmt.Sprintf("%s (%s)", desc, e.Code)
	}
	if e.Type != "" {
		desc = fmt.Sprintf("%s [%s]", desc, e.Type)
	}
	return desc
}

// ChatCompletion 调用 DeepSeek Chat Completion 接口并返回解析结果。
// 核心步骤：校验参数 → 序列化 JSON → 补充认证头发起请求 → 判断状态码并解析成功或错误响应。
func (c *Client) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	// 1. 兜底上下文、校验必要字段，确保不会向 DeepSeek 发送无效请求。
	if c == nil {
		return ChatCompletionResponse{}, fmt.Errorf("deepseek client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if req.Model == "" {
		return ChatCompletionResponse{}, fmt.Errorf("model 字段不能为空")
	}
	if len(req.Messages) == 0 {
		return ChatCompletionResponse{}, fmt.Errorf("messages 至少需要一条消息")
	}

	// 2. 序列化请求体，保持所有字段（含 ExtraFields）进入 JSON。
	body, err := json.Marshal(req)
	if err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	// 3. 拼装 HTTP 请求，补充认证信息与通用头部。
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// 4. 统一读取响应体，便于成功与错误场景共享原始 payload。
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return ChatCompletionResponse{}, c.parseAPIError(resp.StatusCode, rawBody)
	}

	var completion ChatCompletionResponse
	if err := json.Unmarshal(rawBody, &completion); err != nil {
		return ChatCompletionResponse{}, fmt.Errorf("decode response: %w", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(rawBody, &raw); err == nil {
		completion.Raw = raw
	}
	return completion, nil
}

// parseAPIError 将 DeepSeek 的错误包裹解析为 *APIError，方便上层做类型化处理。
func (c *Client) parseAPIError(status int, payload []byte) error {
	type errorEnvelope struct {
		Error struct {
			Message string          `json:"message"`
			Type    string          `json:"type"`
			Code    string          `json:"code"`
			Param   string          `json:"param"`
			Raw     json.RawMessage `json:"-"`
		} `json:"error"`
	}
	if len(payload) == 0 {
		return &APIError{
			StatusCode: status,
			Message:    fmt.Sprintf("deepseek api error: status %d", status),
		}
	}
	var env errorEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return &APIError{
			StatusCode: status,
			Message:    fmt.Sprintf("deepseek api error: status %d, body: %s", status, string(payload)),
			Raw:        payload,
		}
	}
	apiErr := &APIError{
		StatusCode: status,
		Message:    env.Error.Message,
		Type:       env.Error.Type,
		Code:       env.Error.Code,
		Raw:        payload,
	}
	return apiErr
}
