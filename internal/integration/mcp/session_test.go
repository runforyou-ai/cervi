package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

// newSessionTestServer 启动带回声、报错和结构化结果工具的测试 MCP 服务。
func newSessionTestServer(t *testing.T) Config {
	t.Helper()
	server := sdk.NewServer(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	server.AddTool(&sdk.Tool{
		Name: "echo", Description: "回声",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}}, "required": []string{"text"}},
	}, func(_ context.Context, request *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		var arguments struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(request.Params.Arguments, &arguments); err != nil {
			return nil, err
		}
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "echo:" + arguments.Text}}}, nil
	})
	server.AddTool(&sdk.Tool{Name: "broken", Description: "失败", InputSchema: map[string]any{"type": "object"}},
		func(context.Context, *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
			return &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: "库存服务不可用"}}}, nil
		})
	server.AddTool(&sdk.Tool{Name: "structured", Description: "结构化", InputSchema: map[string]any{"type": "object"}},
		func(context.Context, *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
			return &sdk.CallToolResult{StructuredContent: map[string]any{"count": 2}}, nil
		})
	endpoint := httptest.NewServer(sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, nil))
	t.Cleanup(endpoint.Close)
	return Config{URL: endpoint.URL, ServerType: domain.MCPServerTypeStreamableHTTP}
}

// TestSessionToolsAndCall 验证目录读取保留输入 schema，并区分成功、工具报错与协议错误。
func TestSessionToolsAndCall(t *testing.T) {
	ctx := context.Background()
	session, err := Connect(ctx, newSessionTestServer(t))
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	tools, err := session.Tools(ctx)
	if err != nil || len(tools) != 3 {
		t.Fatalf("tools = %+v, err = %v", tools, err)
	}
	echo := tools[1]
	if echo.Name != "echo" || echo.Description != "回声" || !strings.Contains(string(echo.InputSchema), `"required":["text"]`) {
		t.Fatalf("echo tool = %+v", echo)
	}

	result, err := session.Call(ctx, "echo", json.RawMessage(`{"text":"你好"}`))
	if err != nil || result != "echo:你好" {
		t.Fatalf("call result = %q, err = %v", result, err)
	}

	structured, err := session.Call(ctx, "structured", json.RawMessage(`{}`))
	if err != nil || structured != `{"count":2}` {
		t.Fatalf("structured result = %q, err = %v", structured, err)
	}

	if _, err := session.Call(ctx, "broken", json.RawMessage(`{}`)); err == nil || err.Error() != "库存服务不可用" {
		t.Fatalf("tool failure = %v", err)
	}

	_, err = session.Call(ctx, "missing", json.RawMessage(`{}`))
	if _, kind, ok := connectiontest.Details(err); !ok || kind != connectiontest.FailureProtocol {
		t.Fatalf("unknown tool classification: %v", err)
	}
}

// TestConnectUnavailable 验证远端不可达时返回分类后的连接错误。
func TestConnectUnavailable(t *testing.T) {
	_, err := Connect(context.Background(), Config{URL: "http://127.0.0.1:1/mcp", ServerType: domain.MCPServerTypeStreamableHTTP})
	if _, _, ok := connectiontest.Details(err); !ok {
		t.Fatalf("connect error = %v", err)
	}
}
