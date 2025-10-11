package volcengine

import (
	context "context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	arkmodel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/volcengineerr"
)

const defaultBaseURL = "https://ark.cn-beijing.volces.com/api/v3"

// Client 封装与火山引擎 Ark Runtime 的交互逻辑。
type Client struct {
	apiKey  string
	baseURL string
	sdk     *arkruntime.Client
}

// Option 允许自定义 Client 行为。
type Option func(*Client)

// WithBaseURL 设置自定义 Base URL。
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		trimmed := strings.TrimSpace(baseURL)
		if trimmed == "" {
			return
		}
		c.baseURL = strings.TrimRight(trimmed, "/")
	}
}

// NewClient 以 API Key 初始化火山引擎客户端。
// 默认指向华北地域，可使用 Option 覆盖基础地址等参数。
func NewClient(apiKey string, opts ...Option) *Client {
	client := &Client{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: defaultBaseURL,
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// ensureSDK 延迟创建底层 SDK 客户端，避免无意义的实例化。
// 每次调用都会检查 sdk 是否已初始化。
func (c *Client) ensureSDK() {
	if c.sdk != nil {
		return
	}
	options := []arkruntime.ConfigOption{}
	if c.baseURL != "" {
		options = append(options, arkruntime.WithBaseUrl(c.baseURL))
	}
	c.sdk = arkruntime.NewClientWithApiKey(c.apiKey, options...)
}

// ChatCompletion 调用火山引擎 Chat Completion 接口并返回解析结果。
// 会自动拼装消息、构造 SDK 请求并转换异常，方便上层统一处理。
func (c *Client) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	if c == nil {
		return ChatCompletionResponse{}, fmt.Errorf("volcengine client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if c.apiKey == "" {
		return ChatCompletionResponse{}, fmt.Errorf("volcengine api key is empty")
	}
	if strings.TrimSpace(req.Model) == "" {
		return ChatCompletionResponse{}, fmt.Errorf("model 字段不能为空")
	}
	if len(req.Messages) == 0 {
		return ChatCompletionResponse{}, fmt.Errorf("messages 至少需要一条消息")
	}

	c.ensureSDK()

	arkReq := arkmodel.CreateChatCompletionRequest{
		Model:    req.Model,
		Messages: make([]*arkmodel.ChatCompletionMessage, 0, len(req.Messages)),
	}

	for _, msg := range req.Messages {
		content := msg.Content
		role := normalizeRole(msg.Role)
		arkReq.Messages = append(arkReq.Messages, &arkmodel.ChatCompletionMessage{
			Role: role,
			Content: &arkmodel.ChatCompletionMessageContent{
				StringValue: volcengine.String(content),
			},
		})
	}

	if req.MaxTokens > 0 {
		arkReq.MaxTokens = volcengine.Int(req.MaxTokens)
	}
	if req.Temperature > 0 {
		arkReq.Temperature = volcengine.Float32(float32(req.Temperature))
	}
	if req.TopP > 0 {
		arkReq.TopP = volcengine.Float32(float32(req.TopP))
	}
	if req.PresencePenalty > 0 {
		arkReq.PresencePenalty = volcengine.Float32(float32(req.PresencePenalty))
	}
	if req.FrequencyPenalty > 0 {
		arkReq.FrequencyPenalty = volcengine.Float32(float32(req.FrequencyPenalty))
	}
	if len(req.Stop) > 0 {
		arkReq.Stop = append(arkReq.Stop, req.Stop...)
	}

	resp, err := c.sdk.CreateChatCompletion(ctx, arkReq)
	if err != nil {
		if rf, ok := err.(volcengineerr.RequestFailure); ok {
			return ChatCompletionResponse{}, &APIError{
				StatusCode: rf.StatusCode(),
				Code:       rf.Code(),
				Message:    rf.Message(),
			}
		}
		return ChatCompletionResponse{}, fmt.Errorf("volcengine chat completion: %w", err)
	}

	return convertResponse(resp)
}

// normalizeRole 将调用方传入的角色名称转换为方舟 SDK 识别的常量。
func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "system":
		return arkmodel.ChatMessageRoleSystem
	case "assistant":
		return arkmodel.ChatMessageRoleAssistant
	case "tool":
		return arkmodel.ChatMessageRoleTool
	default:
		return arkmodel.ChatMessageRoleUser
	}
}

// convertResponse 将方舟返回的结构体转换为本地定义的数据模型。
func convertResponse(resp arkmodel.ChatCompletionResponse) (ChatCompletionResponse, error) {
	converted := ChatCompletionResponse{
		ID:      resp.ID,
		Object:  resp.Object,
		Created: resp.Created,
		Model:   resp.Model,
	}

	if len(resp.Choices) > 0 {
		converted.Choices = make([]ChatCompletionChoice, 0, len(resp.Choices))
		for _, choice := range resp.Choices {
			content := ""
			if choice.Message.Content != nil && choice.Message.Content.StringValue != nil {
				content = *choice.Message.Content.StringValue
			}
			reasoning := ""
			if choice.Message.ReasoningContent != nil {
				reasoning = *choice.Message.ReasoningContent
			}
			converted.Choices = append(converted.Choices, ChatCompletionChoice{
				Index: choice.Index,
				Message: ChatMessage{
					Role:             choice.Message.Role,
					Content:          content,
					ReasoningContent: reasoning,
				},
				FinishReason: string(choice.FinishReason),
			})
		}
	}

	usageData := resp.Usage
	if usageData.PromptTokens != 0 || usageData.CompletionTokens != 0 || usageData.TotalTokens != 0 || usageData.PromptTokensDetails.CachedTokens != 0 || usageData.CompletionTokensDetails.ReasoningTokens != 0 {
		usage := &ChatCompletionUsage{
			PromptTokens:     usageData.PromptTokens,
			CompletionTokens: usageData.CompletionTokens,
			TotalTokens:      usageData.TotalTokens,
			CachedTokens:     usageData.PromptTokensDetails.CachedTokens,
			ReasoningTokens:  usageData.CompletionTokensDetails.ReasoningTokens,
		}
		usage.ProvisionedPromptTokens = usageData.PromptTokensDetails.ProvisionedTokens
		usage.ProvisionedCompTokens = usageData.CompletionTokensDetails.ProvisionedTokens
		converted.Usage = usage
	}

	if rawBytes, err := json.Marshal(resp); err == nil {
		var raw map[string]any
		if err := json.Unmarshal(rawBytes, &raw); err == nil {
			converted.Raw = raw
		}
	}

	return converted, nil
}
