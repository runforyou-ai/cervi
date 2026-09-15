//go:build server

package agentruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
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
func (m *mcpToolChatModel) Generate(_ context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.AgenticMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.toolInfos = model.GetCommonOptions(&model.Options{}, opts...).Tools
	if m.calls == 1 {
		return assistantReply("先查工单", &schema.FunctionToolCall{
			CallID: "mcp-call-1", Name: "lookup_ticket", Arguments: `{"id":"T-9"}`,
		}), nil
	}
	m.toolReply = messageText(input[len(input)-1])
	return assistantReply("查询结果：" + m.toolReply), nil
}

// Stream 以单个分片返回当前测试步骤的模型输出。
func (m *mcpToolChatModel) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	return singleChunkStream(m.Generate(ctx, input, opts...))
}

// TestRuntimeCallsMCPTools 验证注册后的长连接会话仍可调用工具，不可用的服务被跳过，内置同名工具保留。
func TestRuntimeCallsMCPTools(t *testing.T) {
	calculator, err := newCalculatorTool()
	if err != nil {
		t.Fatal(err)
	}
	chatModel := &mcpToolChatModel{}
	runtime := &EinoRuntime{
		newModel: func(context.Context, ModelConfig) (model.AgenticModel, error) { return chatModel, nil },
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
	var remote *schema.ToolInfo
	for _, info := range chatModel.toolInfos {
		names = append(names, info.Name)
		if info.Name == "lookup_ticket" {
			remote = info
		}
	}
	if strings.Count(strings.Join(names, ","), "calculator") != 1 || remote == nil {
		t.Fatalf("tool names = %v", names)
	}
	parameters, err := remote.ParamsOneOf.ToJSONSchema()
	if err != nil || parameters.Required[0] != "id" {
		t.Fatalf("remote tool schema = %+v, err = %v", parameters, err)
	}
}

type offloadReadingChatModel struct {
	mu         sync.Mutex
	calls      int
	notice     string
	fileResult string
}

// Generate 先调用大结果工具，再读回被转存的完整内容。
func (m *offloadReadingChatModel) Generate(_ context.Context, input []*schema.AgenticMessage, _ ...model.Option) (*schema.AgenticMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	switch m.calls {
	case 1:
		return assistantReply("先抓取", &schema.FunctionToolCall{
			CallID: "dump-call", Name: "dump", Arguments: `{}`,
		}), nil
	case 2:
		m.notice = messageText(input[len(input)-1])
		path := ""
		for _, field := range strings.Fields(m.notice) {
			if strings.HasPrefix(field, "/trunc/") {
				path = field
				break
			}
		}
		arguments, err := json.Marshal(map[string]string{"file_path": path})
		if err != nil {
			return nil, err
		}
		return assistantReply("读取完整内容", &schema.FunctionToolCall{
			CallID: "read-call", Name: offloadedResultToolName, Arguments: string(arguments),
		}), nil
	default:
		m.fileResult = messageText(input[len(input)-1])
		return assistantReply("已读取"), nil
	}
}

// Stream 以单个分片返回当前测试步骤的模型输出。
func (m *offloadReadingChatModel) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	return singleChunkStream(m.Generate(ctx, input, opts...))
}

// TestLargeToolResultOffloaded 验证过大的工具结果转存后只向模型提供预览，完整内容仍可由读回工具取得。
func TestLargeToolResultOffloaded(t *testing.T) {
	full := strings.Repeat("字", minToolResultOffloadBytes)
	server := sdk.NewServer(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	server.AddTool(&sdk.Tool{Name: "dump", Description: "大结果", InputSchema: map[string]any{"type": "object"}},
		func(context.Context, *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
			return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: full}}}, nil
		})
	endpoint := httptest.NewServer(sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, nil))
	defer endpoint.Close()

	var logOutput bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logOutput, nil)))
	t.Cleanup(func() { slog.SetDefault(originalLogger) })

	chatModel := &offloadReadingChatModel{}
	runtime := &EinoRuntime{newModel: func(context.Context, ModelConfig) (model.AgenticModel, error) { return chatModel, nil }}
	feed := &testInputFeed{}
	feed.appendUser("抓一下这个页面")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := runtime.Run(ctx, RunRequest{
		RunID: "offload-run", Name: "test-agent", MaxIterations: 5, MaxTurns: 2,
		MCPServers: []MCPServer{{Name: "大结果服务", Config: mcp.Config{URL: endpoint.URL, ServerType: domain.MCPServerTypeStreamableHTTP}}},
	}, feed)
	if err != nil || result.Content != "已读取" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	// 运行过程记录保存转存后的预览，不落全文。
	for _, block := range result.Blocks {
		if block.Payload.ToolCall != nil && block.Payload.ToolCall.Name == "dump" &&
			block.Payload.ToolCall.Result != nil && len(*block.Payload.ToolCall.Result) >= len(full) {
			t.Fatalf("recorded dump result bytes = %d", len(*block.Payload.ToolCall.Result))
		}
	}
	chatModel.mu.Lock()
	defer chatModel.mu.Unlock()
	if len(chatModel.notice) >= len(full) || !strings.Contains(chatModel.notice, "/trunc/") {
		t.Fatalf("offload notice = %q", chatModel.notice)
	}
	if !strings.Contains(chatModel.fileResult, "字") {
		t.Fatalf("offloaded result read back = %q", chatModel.fileResult)
	}
	if !strings.Contains(logOutput.String(), `"msg":"Agent 工具结果过大，已转存并保留预览"`) ||
		!strings.Contains(logOutput.String(), `"tool_name":"dump"`) {
		t.Fatalf("offload log = %s", logOutput.String())
	}
}
