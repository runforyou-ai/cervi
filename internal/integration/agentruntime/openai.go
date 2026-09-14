//go:build server

package agentruntime

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"time"

	openai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
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

// toolArgumentsNormalizer 把工具调用的空参数补为空 JSON 对象，请求体按 omitempty 序列化时保留 arguments 字段。
type toolArgumentsNormalizer struct {
	adk.BaseChatModelAgentMiddleware
}

// BeforeModelRewriteState 在每次模型调用前替换空工具参数，原消息保持不变。
func (*toolArgumentsNormalizer) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, _ *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	for i, message := range state.Messages {
		if !slices.ContainsFunc(message.ToolCalls, func(call schema.ToolCall) bool { return call.Function.Arguments == "" }) {
			continue
		}
		normalized := *message
		normalized.ToolCalls = slices.Clone(message.ToolCalls)
		for j := range normalized.ToolCalls {
			if normalized.ToolCalls[j].Function.Arguments == "" {
				normalized.ToolCalls[j].Function.Arguments = "{}"
			}
		}
		state.Messages[i] = &normalized
	}
	return ctx, state, nil
}
