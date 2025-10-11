package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/infra/model/deepseek"
	"electron-go-app/backend/internal/infra/security"

	"gorm.io/gorm"
)

const (
	// defaultDeepSeekModel 当用户未填写具体模型时的默认型号。
	defaultDeepSeekModel = "deepseek-chat"
)

// InvokeDeepSeekChatCompletion 读取用户凭据并调用 DeepSeek 的 Chat Completion 接口。
// 关键流程：从数据库查凭据→解密 API Key→合并请求/扩展参数→构造客户端发起请求。
func (s *Service) InvokeDeepSeekChatCompletion(ctx context.Context, userID uint, modelKey string, req deepseek.ChatCompletionRequest) (deepseek.ChatCompletionResponse, error) {
	credential, err := s.repo.FindByModelKey(ctx, userID, modelKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return deepseek.ChatCompletionResponse{}, ErrCredentialNotFound
		}
		return deepseek.ChatCompletionResponse{}, fmt.Errorf("find credential: %w", err)
	}
	return s.invokeDeepSeek(ctx, credential, req)
}

// TestDeepSeekConnection 尝试使用指定凭据发起一次调用，并记录最新验证时间。
func (s *Service) TestDeepSeekConnection(ctx context.Context, userID, id uint, req deepseek.ChatCompletionRequest) (deepseek.ChatCompletionResponse, error) {
	credential, err := s.repo.FindByID(ctx, userID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return deepseek.ChatCompletionResponse{}, ErrCredentialNotFound
		}
		return deepseek.ChatCompletionResponse{}, fmt.Errorf("find credential: %w", err)
	}

	// 与在线调用共享调用链，确保所有校验逻辑一致。
	resp, err := s.invokeDeepSeek(ctx, credential, req)
	if err != nil {
		return deepseek.ChatCompletionResponse{}, err
	}

	// 成功后刷新最近验证时间，帮助前端提示凭据连通状态。
	now := time.Now()
	credential.LastVerifiedAt = &now
	if updateErr := s.repo.Update(ctx, credential); updateErr != nil {
		return deepseek.ChatCompletionResponse{}, fmt.Errorf("update last_verified_at: %w", updateErr)
	}
	return resp, nil
}

// prepareDeepSeekRequest 会根据用户请求与持久化配置补齐模型参数，确保模型字段永远有值。
func prepareDeepSeekRequest(req deepseek.ChatCompletionRequest, credential *domain.UserModelCredential) (deepseek.ChatCompletionRequest, error) {
	request := req
	request.Model = strings.TrimSpace(request.Model)
	if request.Model == "" {
		request.Model = strings.TrimSpace(credential.ModelKey)
	}
	if request.Model == "" {
		request.Model = defaultDeepSeekModel
	}
	// 深拷贝 ExtraFields，避免直接修改调用方提供的映射。
	request.ExtraFields = cloneMap(req.ExtraFields)

	if err := mergeExtraConfig(&request, credential.ExtraConfig); err != nil {
		return deepseek.ChatCompletionRequest{}, err
	}
	return request, nil
}

// mergeExtraConfig 将数据库中的 extra_config 映射到请求体，只有调用方未显式设置的字段才会被填充。
func mergeExtraConfig(request *deepseek.ChatCompletionRequest, extraJSON string) error {
	if strings.TrimSpace(extraJSON) == "" {
		return nil
	}

	extra := map[string]any{}
	if err := json.Unmarshal([]byte(extraJSON), &extra); err != nil {
		return fmt.Errorf("decode extra_config: %w", err)
	}

	for key, value := range extra {
		switch strings.ToLower(key) {
		case "model":
			if strings.TrimSpace(request.Model) == "" {
				if v, ok := value.(string); ok && v != "" {
					request.Model = strings.TrimSpace(v)
				}
			}
		case "max_tokens":
			if request.MaxTokens == 0 {
				if v, ok := asInt(value); ok {
					request.MaxTokens = v
				}
			}
		case "temperature":
			if request.Temperature == 0 {
				if v, ok := asFloat(value); ok {
					request.Temperature = v
				}
			}
		case "top_p":
			if request.TopP == 0 {
				if v, ok := asFloat(value); ok {
					request.TopP = v
				}
			}
		case "presence_penalty":
			if request.PresencePenalty == 0 {
				if v, ok := asFloat(value); ok {
					request.PresencePenalty = v
				}
			}
		case "frequency_penalty":
			if request.FrequencyPenalty == 0 {
				if v, ok := asFloat(value); ok {
					request.FrequencyPenalty = v
				}
			}
		case "response_format":
			if request.ResponseFormat == nil {
				if v, ok := value.(map[string]any); ok {
					request.ResponseFormat = v
				}
			}
		case "stop":
			if request.Stop == nil {
				request.Stop = value
			}
		case "stream":
			if !request.Stream {
				if v, ok := asBool(value); ok {
					request.Stream = v
				}
			}
		case "stream_options":
			if request.StreamOptions == nil {
				if v, ok := value.(map[string]any); ok {
					request.StreamOptions = v
				}
			}
		case "tools":
			if request.Tools == nil {
				request.Tools = value
			}
		case "tool_choice":
			if request.ToolChoice == nil {
				request.ToolChoice = value
			}
		case "logprobs":
			if !request.Logprobs {
				if v, ok := asBool(value); ok {
					request.Logprobs = v
				}
			}
		case "top_logprobs":
			if request.TopLogprobs == nil {
				request.TopLogprobs = value
			}
		default:
			if request.ExtraFields == nil {
				request.ExtraFields = map[string]any{}
			}
			if _, exists := request.ExtraFields[key]; !exists {
				request.ExtraFields[key] = value
			}
		}
	}
	return nil
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func asInt(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	case int:
		return v, true
	case int64:
		return int(v), true
	case json.Number:
		i64, err := v.Int64()
		if err == nil {
			return int(i64), true
		}
	}
	return 0, false
}

func asFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f64, err := v.Float64()
		if err == nil {
			return f64, true
		}
	}
	return 0, false
}

func asBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case float64:
		return v != 0, true
	case float32:
		return v != 0, true
	case int:
		return v != 0, true
	case int64:
		return v != 0, true
	case string:
		lower := strings.ToLower(strings.TrimSpace(v))
		if lower == "true" || lower == "1" {
			return true, true
		}
		if lower == "false" || lower == "0" {
			return false, true
		}
	}
	return false, false
}

func (s *Service) invokeDeepSeek(ctx context.Context, credential *domain.UserModelCredential, req deepseek.ChatCompletionRequest) (deepseek.ChatCompletionResponse, error) {
	if credential == nil {
		return deepseek.ChatCompletionResponse{}, ErrCredentialNotFound
	}
	if !strings.EqualFold(strings.TrimSpace(credential.Provider), "deepseek") {
		return deepseek.ChatCompletionResponse{}, ErrUnsupportedProvider
	}
	if strings.EqualFold(credential.Status, "disabled") {
		return deepseek.ChatCompletionResponse{}, ErrCredentialDisabled
	}

	apiKeyPlain, err := security.Decrypt(credential.APIKeyCipher)
	if err != nil {
		return deepseek.ChatCompletionResponse{}, fmt.Errorf("decrypt api key: %w", err)
	}

	prepared, err := prepareDeepSeekRequest(req, credential)
	if err != nil {
		return deepseek.ChatCompletionResponse{}, err
	}

	baseURL := strings.TrimSpace(credential.BaseURL)
	client := deepseek.NewClient(string(apiKeyPlain), deepseek.WithBaseURL(baseURL))

	return client.ChatCompletion(ctx, prepared)
}
