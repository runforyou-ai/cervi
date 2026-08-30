//go:build server

package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	openai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

const modelRequestTimeout = 2 * time.Minute

type modelFactory func(context.Context, ModelConfig) (model.ToolCallingChatModel, error)

// newOpenAICompatibleModel 使用 eino-ext 创建 OpenAI 兼容模型组件。
func newOpenAICompatibleModel(ctx context.Context, config ModelConfig) (model.ToolCallingChatModel, error) {
	baseURL, err := compatibleBaseURL(config.Brand, config.BaseURL)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{
		Timeout: modelRequestTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	componentConfig := &openai.ChatModelConfig{
		APIKey: config.APIKey, BaseURL: baseURL, Model: config.Identifier,
		HTTPClient: httpClient,
	}
	if config.MaxOutputTokens > 0 {
		if config.Brand == "openai" {
			componentConfig.MaxCompletionTokens = &config.MaxOutputTokens
		} else {
			componentConfig.MaxTokens = &config.MaxOutputTokens
		}
	}
	chatModel, err := openai.NewChatModel(ctx, componentConfig)
	if err != nil {
		return nil, fmt.Errorf("create OpenAI-compatible Eino model: %w", err)
	}
	return chatModel, nil
}

// compatibleBaseURL 把供应商地址规范为 Chat Completions 兼容入口。
func compatibleBaseURL(brand, value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("parse model base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("model base URL must include scheme and host")
	}
	if brand != "alibaba" {
		return strings.TrimSuffix(parsed.String(), "/"), nil
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	switch {
	case strings.HasSuffix(path, "/compatible-mode/v1"):
	case strings.HasSuffix(path, "/api/v1"):
		path = strings.TrimSuffix(path, "/api/v1") + "/compatible-mode/v1"
	default:
		path += "/compatible-mode/v1"
	}
	parsed.Path = path
	parsed.RawPath = ""
	return parsed.String(), nil
}
