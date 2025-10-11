package deepseek

import "encoding/json"

// ChatMessage 表示与 DeepSeek 模型交互的单条对话消息。
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest 对应 DeepSeek Chat Completion API 的请求体。
type ChatCompletionRequest struct {
	Model            string         `json:"model"`
	Messages         []ChatMessage  `json:"messages"`
	MaxTokens        int            `json:"max_tokens,omitempty"`
	Temperature      float64        `json:"temperature,omitempty"`
	TopP             float64        `json:"top_p,omitempty"`
	PresencePenalty  float64        `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64        `json:"frequency_penalty,omitempty"`
	ResponseFormat   map[string]any `json:"response_format,omitempty"`
	Stop             any            `json:"stop,omitempty"`
	Stream           bool           `json:"stream,omitempty"`
	StreamOptions    map[string]any `json:"stream_options,omitempty"`
	Tools            any            `json:"tools,omitempty"`
	ToolChoice       any            `json:"tool_choice,omitempty"`
	Logprobs         bool           `json:"logprobs,omitempty"`
	TopLogprobs      any            `json:"top_logprobs,omitempty"`
	ExtraFields      map[string]any `json:"-"`
}

// MarshalJSON 将 ExtraFields 合并到默认字段中，便于在需要时追加扩展参数。
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

// ChatCompletionResponse 映射 DeepSeek 返回的响应结构。
type ChatCompletionResponse struct {
	ID                string                 `json:"id"`
	Object            string                 `json:"object"`
	Created           int64                  `json:"created"`
	Model             string                 `json:"model"`
	Choices           []ChatCompletionChoice `json:"choices"`
	Usage             *ChatCompletionUsage   `json:"usage,omitempty"`
	SystemFingerprint string                 `json:"system_fingerprint,omitempty"`
	Raw               map[string]any         `json:"-"`
}

// ChatCompletionChoice 描述一次对话中模型返回的单个选项。
type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	Logprobs     any         `json:"logprobs"`
	FinishReason string      `json:"finish_reason"`
}

// ChatCompletionUsage 统计 token 消耗情况。
type ChatCompletionUsage struct {
	PromptTokens           int64                  `json:"prompt_tokens"`
	CompletionTokens       int64                  `json:"completion_tokens"`
	TotalTokens            int64                  `json:"total_tokens"`
	PromptTokensDetails    map[string]any         `json:"prompt_tokens_details,omitempty"`
	PromptCacheHitTokens   int64                  `json:"prompt_cache_hit_tokens,omitempty"`
	PromptCacheMissTokens  int64                  `json:"prompt_cache_miss_tokens,omitempty"`
	CompletionTokensByType map[string]json.Number `json:"completion_tokens_by_type,omitempty"`
}
