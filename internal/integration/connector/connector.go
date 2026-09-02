// Package connector 实现外部系统连接器的只读访问适配。
package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

// HTTPDoer 定义连接器 HTTP 请求需要的最小客户端契约。
type HTTPDoer = connectiontest.HTTPDoer

// Config 定义创建连接器探测器需要的配置。
type Config struct {
	Type   domain.IntegrationConnectionType
	APIURL string
	APIKey string
}

// Factory 根据配置创建连接器探测器。
type Factory func(Config) (connectiontest.Probe, error)

// Registry 按连接器类型选择探测适配器。
type Registry struct {
	factories map[domain.IntegrationConnectionType]Factory
}

// NewRegistry 创建内置连接器注册表。
func NewRegistry(client HTTPDoer) *Registry {
	return &Registry{factories: map[domain.IntegrationConnectionType]Factory{
		domain.IntegrationConnectionTypeDify: newDifyFactory(client),
		domain.IntegrationConnectionTypeN8N:  newN8NFactory(client),
	}}
}

// NewProbe 返回指定类型的连接器探测器。
func (r *Registry) NewProbe(config Config) (connectiontest.Probe, error) {
	factory, ok := r.factories[config.Type]
	if !ok {
		return nil, connectiontest.NewError(
			connectiontest.StageCapability,
			connectiontest.FailureInvalidConfig,
			fmt.Errorf("unsupported connector type %q", config.Type),
		)
	}
	return factory(config)
}

type endpoint struct {
	request  *http.Request
	validate func(io.Reader) error
}

// httpProbe 使用主接口探测，并按需尝试兼容接口。
type httpProbe struct {
	client    HTTPDoer
	primary   endpoint
	fallbacks []endpoint
}

// Run 执行连接器 HTTP 探测。
func (p *httpProbe) Run(ctx context.Context) error {
	err := connectiontest.ReadHTTPResponse(ctx, p.client, p.primary.request, p.primary.validate)
	for _, candidate := range p.fallbacks {
		if err == nil {
			return nil
		}
		// 判断当前失败是否允许尝试兼容接口。
		_, kind, classified := connectiontest.Details(err)
		supportsFallback := classified && (kind == connectiontest.FailureUnauthorized ||
			kind == connectiontest.FailureForbidden ||
			kind == connectiontest.FailureNotFound)
		if !supportsFallback {
			return err
		}
		err = connectiontest.ReadHTTPResponse(ctx, p.client, candidate.request, candidate.validate)
	}
	return err
}

// newDifyFactory 创建同时兼容 Dify 应用密钥和知识库密钥的探测器。
func newDifyFactory(client HTTPDoer) Factory {
	return func(config Config) (connectiontest.Probe, error) {
		appRequest, err := newRequest(config.APIURL, "info", config.APIKey, "Authorization", "Bearer ")
		if err != nil {
			return nil, connectiontest.InvalidConfigError(err)
		}
		knowledgeRequest, err := newRequest(config.APIURL, "datasets", config.APIKey, "Authorization", "Bearer ")
		if err != nil {
			return nil, connectiontest.InvalidConfigError(err)
		}
		return &httpProbe{
			client:    client,
			primary:   endpoint{request: appRequest, validate: validateDifyApp},
			fallbacks: []endpoint{{request: knowledgeRequest, validate: connectiontest.ValidateDataList}},
		}, nil
	}
}

// newN8NFactory 创建 n8n 工作流列表探测器。
func newN8NFactory(client HTTPDoer) Factory {
	return func(config Config) (connectiontest.Probe, error) {
		request, err := newRequest(config.APIURL, "api/v1/workflows", config.APIKey, "X-N8N-API-KEY", "")
		if err != nil {
			return nil, connectiontest.InvalidConfigError(err)
		}
		query := request.URL.Query()
		query.Set("limit", "1")
		request.URL.RawQuery = query.Encode()
		return &httpProbe{
			client:  client,
			primary: endpoint{request: request, validate: connectiontest.ValidateDataList},
		}, nil
	}
}

// newRequest 创建连接器探测请求。
func newRequest(baseURL, path, apiKey, header, prefix string) (*http.Request, error) {
	requestURL, err := connectiontest.AppendPath(baseURL, path)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set(header, prefix+apiKey)
	return request, nil
}

// validateDifyApp 校验 Dify 应用信息的最小响应契约。
func validateDifyApp(reader io.Reader) error {
	var payload struct {
		Name string `json:"name"`
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(reader).Decode(&payload); err != nil {
		return err
	}
	if strings.TrimSpace(payload.Name) == "" || strings.TrimSpace(payload.Mode) == "" {
		return errors.New("Dify app response does not contain name and mode")
	}
	return nil
}
