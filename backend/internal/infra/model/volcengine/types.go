package volcengine

import "encoding/json"

// ChatMessage 表示向火山引擎发起请求或从响应中解析出的单条消息。
type ChatMessage struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

// ChatCompletionRequest 封装火山引擎聊天补全 API 所需的参数。
type ChatCompletionRequest struct {
	Model            string         `json:"model"`
	Messages         []ChatMessage  `json:"messages"`
	MaxTokens        int            `json:"max_tokens,omitempty"`
	Temperature      float64        `json:"temperature,omitempty"`
	TopP             float64        `json:"top_p,omitempty"`
	PresencePenalty  float64        `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64        `json:"frequency_penalty,omitempty"`
	Stop             []string       `json:"stop,omitempty"`
	ResponseFormat   map[string]any `json:"response_format,omitempty"`
	ExtraFields      map[string]any `json:"-"`
}

// MarshalJSON 将 ExtraFields 合并回标准请求结构，便于扩展。
func (r ChatCompletionRequest) MarshalJSON() ([]byte, error) {
	type alias ChatCompletionRequest
	payload := map[string]any{}

	base, err := json.Marshal(alias(r))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(base, &payload); err != nil {
		return nil, err
	}
	if r.ExtraFields != nil {
		for k, v := range r.ExtraFields {
			if _, exists := payload[k]; !exists {
				payload[k] = v
			}
		}
	}
	return json.Marshal(payload)
}

// ChatCompletionResponse 映射火山引擎返回的补全结果。
type ChatCompletionResponse struct {
	ID          string                 `json:"id"`
	Object      string                 `json:"object"`
	Created     int64                  `json:"created"`
	Model       string                 `json:"model"`
	ServiceTier string                 `json:"service_tier,omitempty"` // 方舟会回传服务等级，便于排障
	Choices     []ChatCompletionChoice `json:"choices"`
	Usage       *ChatCompletionUsage   `json:"usage,omitempty"`
	Raw         map[string]any         `json:"-"`
}

// ChatCompletionChoice 描述一次补全中的单个候选。
type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatCompletionUsage 统计 Token 消耗信息。
type ChatCompletionUsage struct {
	PromptTokens            int  `json:"prompt_tokens"`
	CompletionTokens        int  `json:"completion_tokens"`
	TotalTokens             int  `json:"total_tokens"`
	CachedTokens            int  `json:"cached_tokens"`
	ProvisionedPromptTokens *int // 火山引擎特有的预留 token，指针避免空值误传
	ReasoningTokens         int  // Doubao 会返回 reasoning_tokens，方便前端标记
	ProvisionedCompTokens   *int
}

// APIError 封装火山引擎返回的错误信息，便于响应统一化。
type APIError struct {
	StatusCode int    `json:"status_code"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

// Error 实现 error 接口。
func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Code != "" {
		return e.Message + " (" + e.Code + ")"
	}
	return e.Message
}
