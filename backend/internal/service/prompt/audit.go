package prompt

import (
	"context"
	"errors"
	"fmt"
	"strings"

	modeldomain "electron-go-app/backend/internal/infra/model/deepseek"
)

const defaultAuditModelKey = "deepseek-chat"

// AuditConfig 描述内容审核使用的模型配置，支持注入自定义调度器便于测试。
type AuditConfig struct {
	Enabled  bool
	Provider string
	ModelKey string
	APIKey   string
	BaseURL  string
	Invoker  ModelInvoker
}

// normalize 返回修剪空白后的配置拷贝。
func (cfg AuditConfig) normalize() AuditConfig {
	cfg.Provider = strings.TrimSpace(cfg.Provider)
	cfg.ModelKey = strings.TrimSpace(cfg.ModelKey)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	return cfg
}

// buildAuditInvoker 根据配置构造审核模型调用器，若未启用则返回空。
func buildAuditInvoker(cfg AuditConfig) (ModelInvoker, string, error) {
	cfg = cfg.normalize()
	if !cfg.Enabled {
		return nil, "", nil
	}
	if cfg.Invoker != nil {
		return cfg.Invoker, cfg.ModelKey, nil
	}
	provider := strings.ToLower(cfg.Provider)
	if provider == "" {
		provider = "deepseek"
	}
	if cfg.ModelKey == "" {
		cfg.ModelKey = defaultAuditModelKey
	}
	switch provider {
	case "deepseek":
		if cfg.APIKey == "" {
			return nil, "", errors.New("审核模型未配置 API Key")
		}
		opts := []modeldomain.Option{}
		if cfg.BaseURL != "" {
			opts = append(opts, modeldomain.WithBaseURL(cfg.BaseURL))
		}
		client := modeldomain.NewClient(cfg.APIKey, opts...)
		invoker := &deepSeekAuditInvoker{
			client:       client,
			defaultModel: cfg.ModelKey,
		}
		return invoker, cfg.ModelKey, nil
	default:
		return nil, "", fmt.Errorf("暂不支持的审核模型提供方: %s", cfg.Provider)
	}
}

// deepSeekAuditInvoker 直接调用 DeepSeek Chat Completion 作为审核模型。
type deepSeekAuditInvoker struct {
	client       *modeldomain.Client
	defaultModel string
}

// InvokeChatCompletion 调用 DeepSeek 审核模型，优先使用传入的 modelKey。
func (d *deepSeekAuditInvoker) InvokeChatCompletion(ctx context.Context, _ uint, modelKey string, req modeldomain.ChatCompletionRequest) (modeldomain.ChatCompletionResponse, error) {
	if d == nil || d.client == nil {
		return modeldomain.ChatCompletionResponse{}, errors.New("审核模型客户端未初始化")
	}
	key := strings.TrimSpace(modelKey)
	if key == "" {
		key = strings.TrimSpace(d.defaultModel)
	}
	if key == "" {
		return modeldomain.ChatCompletionResponse{}, errors.New("审核模型缺少标识")
	}
	request := req
	request.Model = key
	return d.client.ChatCompletion(ctx, request)
}
