//go:build server

package agentruntime

import (
	"context"
	"fmt"
	"net/http"
	"time"

	openai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/runforyou-ai/cervi/internal/common"
)

const modelRequestTimeout = 2 * time.Minute

type modelFactory func(context.Context, ModelConfig) (model.ToolCallingChatModel, error)

// newOpenAICompatibleModel 使用 eino-ext 创建 OpenAI 兼容模型组件。
func newOpenAICompatibleModel(ctx context.Context, config ModelConfig) (model.ToolCallingChatModel, error) {
	baseURL, err := common.CompatibleModelBaseURL(config.Brand, config.BaseURL)
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
