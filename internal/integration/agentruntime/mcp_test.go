//go:build server

package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/mcp"
)

// newMCPTestServer 启动提供工单查询和同名 calculator 工具的 SSE 测试 MCP 服务。
func newMCPTestServer(t *testing.T) MCPServer {
	t.Helper()
	server := sdk.NewServer(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	server.AddTool(&sdk.Tool{
		Name: "lookup_ticket", Description: "按编号查询工单",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}}, "required": []string{"id"}},
	}, func(_ context.Context, request *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		var arguments struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(request.Params.Arguments, &arguments); err != nil {
			return nil, err
		}
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "工单 " + arguments.ID + " 已完成"}}}, nil
	})
	server.AddTool(&sdk.Tool{Name: "calculator", Description: "远端同名工具", InputSchema: map[string]any{"type": "object"}},
		func(context.Context, *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
			t.Error("built-in tool must win over the remote tool with the same name")
			return &sdk.CallToolResult{}, nil
		})
	endpoint := httptest.NewServer(sdk.NewSSEHandler(func(*http.Request) *sdk.Server { return server }, nil))
	t.Cleanup(endpoint.Close)
	return MCPServer{Name: "工单系统", Config: mcp.Config{URL: endpoint.URL, ServerType: domain.MCPServerTypeSSE}}
}

type mcpToolChatModel struct {
	mu        sync.Mutex
	calls     int
	toolInfos []*schema.ToolInfo
	toolReply string
}

// Generate 记录本次可用工具，首次调用远程工具，随后把工具结果作为最终回复。
func (m *mcpToolChatModel) Generate(_ context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.toolInfos = model.GetCommonOptions(&model.Options{}, opts...).Tools
	if m.calls == 1 {
		return schema.AssistantMessage("先查工单", []schema.ToolCall{{
			ID: "mcp-call-1", Type: "function",
			Function: schema.FunctionCall{Name: "lookup_ticket", Arguments: `{"id":"T-9"}`},
		}}), nil
	}
	m.toolReply = input[len(input)-1].Content
	return schema.AssistantMessage("查询结果："+m.toolReply, nil), nil
}

func (m *mcpToolChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("unexpected streaming call")
}

func (m *mcpToolChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// TestRuntimeCallsMCPTools 验证注册后的长连接会话仍可调用工具，不可用的服务被跳过，内置同名工具保留。
func TestRuntimeCallsMCPTools(t *testing.T) {
	calculator, err := newCalculatorTool()
	if err != nil {
		t.Fatal(err)
	}
	chatModel := &mcpToolChatModel{}
	runtime := &EinoRuntime{
		newModel: func(context.Context, ModelConfig) (model.ToolCallingChatModel, error) { return chatModel, nil },
		tools:    []tool.BaseTool{calculator},
	}
	feed := &testInputFeed{}
	feed.appendUser("T-9 处理好了吗")
	unavailable := MCPServer{Name: "离线服务", Config: mcp.Config{URL: "http://127.0.0.1:1/mcp", ServerType: domain.MCPServerTypeStreamableHTTP}}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := runtime.Run(ctx, RunRequest{
		RunID: "mcp-run", Name: "test-agent", MaxTurns: 2,
		MCPServers: []MCPServer{unavailable, newMCPTestServer(t)},
	}, feed)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "查询结果：工单 T-9 已完成" {
		t.Fatalf("result = %#v", result)
	}
	chatModel.mu.Lock()
	defer chatModel.mu.Unlock()
	names := make([]string, 0, len(chatModel.toolInfos))
	for _, info := range chatModel.toolInfos {
		names = append(names, info.Name)
	}
	if strings.Join(names, ",") != "calculator,lookup_ticket" {
		t.Fatalf("tool names = %v", names)
	}
	parameters, err := chatModel.toolInfos[1].ParamsOneOf.ToJSONSchema()
	if err != nil || parameters.Required[0] != "id" {
		t.Fatalf("remote tool schema = %+v, err = %v", parameters, err)
	}
}

// TestMCPToolResultTruncated 验证过长的工具结果按上限截断后返回给模型。
func TestMCPToolResultTruncated(t *testing.T) {
	server := sdk.NewServer(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	server.AddTool(&sdk.Tool{Name: "dump", Description: "大结果", InputSchema: map[string]any{"type": "object"}},
		func(context.Context, *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
			return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: strings.Repeat("字", mcpResultMaxRunes+10)}}}, nil
		})
	endpoint := httptest.NewServer(sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, nil))
	defer endpoint.Close()

	ctx := context.Background()
	tools, releaseSessions := openMCPTools(ctx, "mcp-run", []MCPServer{{
		Name: "大结果服务", Config: mcp.Config{URL: endpoint.URL, ServerType: domain.MCPServerTypeStreamableHTTP},
	}}, map[string]struct{}{})
	defer releaseSessions()
	if len(tools) != 1 {
		t.Fatalf("tools = %d", len(tools))
	}
	result, err := tools[0].(tool.InvokableTool).InvokableRun(ctx, `{}`)
	if err != nil {
		t.Fatal(err)
	}
	if []rune(result)[mcpResultMaxRunes-1] != '字' || !strings.HasSuffix(result, mcpResultTruncated) {
		t.Fatalf("result length = %d", len([]rune(result)))
	}
}

// TestMCPHandshakeTimeout 验证握手超时后跳过该服务，不注册工具也不残留服务端会话。
func TestMCPHandshakeTimeout(t *testing.T) {
	original := mcpHandshakeTimeout
	mcpHandshakeTimeout = 50 * time.Millisecond
	t.Cleanup(func() { mcpHandshakeTimeout = original })

	server := sdk.NewServer(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	server.AddTool(&sdk.Tool{Name: "slow", Description: "迟缓", InputSchema: map[string]any{"type": "object"}},
		func(context.Context, *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
			return &sdk.CallToolResult{}, nil
		})
	handler := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, nil)
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		handler.ServeHTTP(w, r)
	}))
	defer endpoint.Close()

	tools, releaseSessions := openMCPTools(context.Background(), "mcp-run", []MCPServer{{
		Name: "迟缓服务", Config: mcp.Config{URL: endpoint.URL, ServerType: domain.MCPServerTypeStreamableHTTP},
	}}, map[string]struct{}{})
	defer releaseSessions()
	if len(tools) != 0 {
		t.Fatalf("tools = %d", len(tools))
	}
	// 握手超时分支取消会话 context 并关闭迟到返回的会话，服务端不应残留会话。
	deadline := time.Now().Add(3 * time.Second)
	for {
		remaining := 0
		for range server.Sessions() {
			remaining++
		}
		if remaining == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("server sessions left open = %d", remaining)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestMCPToolNameConflictsWithGroupReply 验证远端同名工具不会取代群内结束工具。
func TestMCPToolNameConflictsWithGroupReply(t *testing.T) {
	server := sdk.NewServer(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	server.AddTool(&sdk.Tool{Name: groupReplyToolName, Description: "远端同名结束工具", InputSchema: map[string]any{"type": "object"}},
		func(context.Context, *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
			t.Error("group reply tool must not be replaced by a remote tool")
			return &sdk.CallToolResult{}, nil
		})
	endpoint := httptest.NewServer(sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, nil))
	defer endpoint.Close()

	chatModel := &groupReplyChatModel{}
	runtime := &EinoRuntime{newModel: func(context.Context, ModelConfig) (model.ToolCallingChatModel, error) { return chatModel, nil }}
	feed := &testInputFeed{}
	feed.appendUser("帮我看下")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := runtime.Run(ctx, RunRequest{
		RunID: "mcp-conflict-run", Name: "test-agent", MaxTurns: 2,
		GroupReply: &GroupReplyConfig{},
		MCPServers: []MCPServer{{Name: "冲突服务", Config: mcp.Config{URL: endpoint.URL, ServerType: domain.MCPServerTypeStreamableHTTP}}},
	}, feed)
	if err != nil || result.Outcome != RunOutcomeReply || result.Content != "已看过" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
}

type groupReplyChatModel struct {
	mu    sync.Mutex
	calls int
}

// Generate 先通过结束工具提交群内回复，再结束本轮。
func (m *groupReplyChatModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.calls == 1 {
		return schema.AssistantMessage("", []schema.ToolCall{{
			ID: "group-reply-call-1", Type: "function",
			Function: schema.FunctionCall{Name: groupReplyToolName, Arguments: `{"outcome":"reply","body":"已看过"}`},
		}}), nil
	}
	return schema.AssistantMessage("结束", nil), nil
}

func (m *groupReplyChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("unexpected streaming call")
}

func (m *groupReplyChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
