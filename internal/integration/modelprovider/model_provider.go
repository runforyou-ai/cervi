// Package modelprovider 实现模型服务供应商连接探测适配器。
package modelprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

// HTTPDoer 定义模型服务探测需要的最小 HTTP 客户端契约。
type HTTPDoer = connectiontest.HTTPDoer

// Config 定义创建模型服务探测器需要的强类型配置。
type Config struct {
	Brand  domain.AIProviderBrand
	APIKey string
	APIURL string
}

// Factory 根据强类型配置创建一个供应商探测器。
type Factory func(Config) (connectiontest.Probe, error)

// Registry 按供应商品牌选择连接探测适配器。
type Registry struct {
	factories map[domain.AIProviderBrand]Factory
}

// NewHTTPClient 创建不会跟随重定向泄露凭据的探测客户端。
func NewHTTPClient() *http.Client {
	return connectiontest.NewHTTPClient()
}

// NewRegistry 创建内置供应商品牌注册表。
func NewRegistry(client HTTPDoer) *Registry {
	registry := &Registry{factories: make(map[domain.AIProviderBrand]Factory)}
	registry.Register(domain.AIProviderBrandDeepSeek, newOpenAICompatibleFactory(client))
	registry.Register(domain.AIProviderBrandOpenAI, newOpenAICompatibleFactory(client))
	registry.Register(domain.AIProviderBrandAlibaba, newAlibabaFactory(client))
	return registry
}

// Register 在应用组装阶段注册或替换一个供应商探测器工厂。
func (r *Registry) Register(brand domain.AIProviderBrand, factory Factory) {
	r.factories[brand] = factory
}

// NewProbe 返回指定品牌的连接探测器。
func (r *Registry) NewProbe(config Config) (connectiontest.Probe, error) {
	factory, ok := r.factories[config.Brand]
	if !ok {
		return nil, connectiontest.NewError(
			connectiontest.StageCapability,
			connectiontest.FailureInvalidConfig,
			fmt.Errorf("unsupported model provider brand %q", config.Brand),
		)
	}
	return factory(config)
}

// httpProbe 使用供应商只读接口验证地址、凭据和基本响应契约。
type httpProbe struct {
	client   HTTPDoer
	request  *http.Request
	validate func(io.Reader) error
}

// Run 执行模型服务供应商 HTTP 探测。
func (p *httpProbe) Run(ctx context.Context) error {
	return connectiontest.RunHTTPProbe(ctx, p.client, p.request, p.validate)
}

// newOpenAICompatibleFactory 创建 OpenAI 兼容模型列表探测器工厂。
func newOpenAICompatibleFactory(client HTTPDoer) Factory {
	return func(config Config) (connectiontest.Probe, error) {
		request, err := newRequest(config)
		if err != nil {
			return nil, err
		}
		return &httpProbe{client: client, request: request, validate: connectiontest.ValidateDataList}, nil
	}
}

// newAlibabaFactory 创建阿里云百炼原生模型列表探测器工厂。
func newAlibabaFactory(client HTTPDoer) Factory {
	return func(config Config) (connectiontest.Probe, error) {
		endpoint, err := alibabaModelsURL(config.APIURL)
		if err != nil {
			return nil, connectiontest.InvalidConfigError(err)
		}
		request, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, connectiontest.InvalidConfigError(err)
		}
		setHeaders(request, config.APIKey)
		return &httpProbe{client: client, request: request, validate: validateAlibabaModelList}, nil
	}
}

// newRequest 创建 OpenAI 兼容的模型列表请求。
func newRequest(config Config) (*http.Request, error) {
	requestURL, err := connectiontest.AppendPath(config.APIURL, "models")
	if err != nil {
		return nil, connectiontest.InvalidConfigError(err)
	}
	request, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, connectiontest.InvalidConfigError(err)
	}
	setHeaders(request, config.APIKey)
	return request, nil
}

// setHeaders 设置模型服务探测的通用请求头。
func setHeaders(request *http.Request, apiKey string) {
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+apiKey)
}

// alibabaModelsURL 把 OpenAI 兼容或原生基础地址转换为百炼模型列表地址。
func alibabaModelsURL(baseURL string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	switch {
	case strings.HasSuffix(path, "/compatible-mode/v1"):
		path = strings.TrimSuffix(path, "/compatible-mode/v1") + "/api/v1/models"
	case strings.HasSuffix(path, "/api/v1"):
		path += "/models"
	default:
		path += "/api/v1/models"
	}
	parsed.RawPath = ""
	parsed.Path = path
	return parsed.String(), nil
}

// validateAlibabaModelList 校验阿里云百炼模型列表的最小响应契约。
func validateAlibabaModelList(reader io.Reader) error {
	var payload struct {
		Success bool `json:"success"`
		Output  struct {
			Models json.RawMessage `json:"models"`
		} `json:"output"`
	}
	if err := json.NewDecoder(reader).Decode(&payload); err != nil {
		return err
	}
	if !payload.Success || len(payload.Output.Models) == 0 || payload.Output.Models[0] != '[' {
		return errors.New("model list response does not contain a successful models array")
	}
	return nil
}
