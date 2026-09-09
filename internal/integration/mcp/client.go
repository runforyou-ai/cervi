// Package mcp 实现远程 MCP 服务的连接与工具发现。
package mcp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

// Config 定义远程 MCP 连接配置。
type Config struct {
	URL                string
	ServerType         domain.MCPServerType
	AuthorizationToken string
}

// Discoverer 读取 MCP 服务的完整工具目录。
type Discoverer interface {
	Discover(context.Context, Config) ([]domain.MCPTool, error)
}

// Client 通过官方 SDK 探测并读取工具目录。
type Client struct{ runner *connectiontest.Runner }

// NewClient 创建具有统一超时的 MCP 客户端。
func NewClient() *Client { return &Client{runner: connectiontest.NewRunner(10 * time.Second)} }

// Discover 完成初始化并遍历所有工具分页，结束后关闭会话。
func (c *Client) Discover(ctx context.Context, config Config) ([]domain.MCPTool, error) {
	tools := make([]domain.MCPTool, 0)
	err := c.runner.Run(ctx, connectiontest.Target{
		Category: connectiontest.CategoryMCPServer, Adapter: string(config.ServerType), Location: connectiontest.LocationServer,
	}, connectiontest.ProbeFunc(func(ctx context.Context) error {
		client := connectiontest.NewHTTPClient()
		deadline, _ := ctx.Deadline()
		client.Transport = &authenticatedTransport{token: config.AuthorizationToken, deadline: deadline}
		var transport sdk.Transport
		switch config.ServerType {
		case domain.MCPServerTypeSSE:
			transport = &sdk.SSEClientTransport{Endpoint: config.URL, HTTPClient: client}
		case domain.MCPServerTypeStreamableHTTP:
			transport = &sdk.StreamableClientTransport{Endpoint: config.URL, HTTPClient: client, MaxRetries: -1, DisableStandaloneSSE: true}
		default:
			return connectiontest.NewError(connectiontest.StageConnect, connectiontest.FailureInvalidConfig, nil)
		}
		session, err := sdk.NewClient(&sdk.Implementation{Name: "Cervi", Version: "1.0.0"}, nil).Connect(ctx, transport, nil)
		if err != nil {
			return classifyError(connectiontest.StageConnect, err)
		}
		defer session.Close()
		for item, err := range session.Tools(ctx, nil) {
			if err != nil {
				return classifyError(connectiontest.StageCapability, err)
			}
			tools = append(tools, domain.MCPTool{Name: item.Name, Description: item.Description})
		}
		return nil
	}))
	if err != nil {
		return nil, err
	}
	slices.SortFunc(tools, func(a, b domain.MCPTool) int { return strings.Compare(a.Name, b.Name) })
	return tools, nil
}

// authenticatedTransport 为 MCP 的每个 HTTP 请求附加认证并分类状态错误。
type authenticatedTransport struct {
	token    string
	deadline time.Time
}

// RoundTrip 发送带认证的请求并归一化传输错误。
func (t *authenticatedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	// 将初始化、分页与关闭会话的 HTTP 请求限制在同一次探测期限内。
	ctx, cancel := context.WithDeadline(request.Context(), t.deadline)
	request = request.Clone(ctx)
	if t.token != "" {
		request.Header.Set("Authorization", "Bearer "+t.token)
	}
	response, err := http.DefaultTransport.RoundTrip(request)
	if err != nil {
		cancel()
		return nil, connectiontest.ClassifyTransportError(connectiontest.StageConnect, err)
	}
	if response.StatusCode >= 300 {
		response.Body.Close()
		cancel()
		return nil, connectiontest.HTTPStatusError(response.StatusCode)
	}
	response.Body = &responseBody{ReadCloser: response.Body, cancel: cancel}
	return response, nil
}

// classifyError 区分 MCP 协议错误和网络错误。
func classifyError(stage connectiontest.Stage, err error) error {
	if _, _, ok := connectiontest.Details(err); ok {
		return err
	}
	var rpcError *jsonrpc.Error
	if errors.As(err, &rpcError) {
		return connectiontest.NewError(stage, connectiontest.FailureProtocol, err)
	}
	return connectiontest.ClassifyTransportError(stage, err)
}

// responseBody 在流式响应关闭后释放请求期限。
type responseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

// Close 关闭响应并释放上下文资源。
func (b *responseBody) Close() error {
	defer b.cancel()
	return b.ReadCloser.Close()
}
