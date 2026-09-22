package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/runforyou-ai/cervi/internal/integration/mcp"
)

// mcpHandshakeTimeout 限制建立会话和读取工具目录的时间。
var mcpHandshakeTimeout = 15 * time.Second

// MCPServer 定义本次运行可调用的一个远程 MCP 服务。
type MCPServer struct {
	Name   string
	Config mcp.Config
}

// openMCPTools 连接本次运行绑定的 MCP 服务并注册其工具，返回释放全部会话的函数。
// 服务不可用、目录读取失败、工具参数定义无法解析或工具名称与已注册工具重复时跳过，本次运行在缺少这部分工具的情况下继续。
func openMCPTools(ctx context.Context, runID string, servers []MCPServer, registered map[string]struct{}) ([]tool.BaseTool, func()) {
	releases := make([]func(), 0, len(servers))
	releaseSessions := func() {
		for _, release := range releases {
			release()
		}
	}
	tools := make([]tool.BaseTool, 0)
	for _, server := range servers {
		session, catalog, release, err := openMCPServer(ctx, server)
		if err != nil {
			slog.Warn("MCP 服务不可用，本次运行跳过其工具",
				"agent_run_id", runID, "mcp_server", server.Name, "error", err)
			continue
		}
		accepted := 0
		for _, item := range catalog {
			if _, exists := registered[item.Name]; exists {
				slog.Warn("MCP 工具名称与已注册工具重复，跳过该工具",
					"agent_run_id", runID, "mcp_server", server.Name, "tool_name", item.Name)
				continue
			}
			info, err := mcpToolInfo(item)
			if err != nil {
				slog.Warn("MCP 工具参数定义无法解析，跳过该工具",
					"agent_run_id", runID, "mcp_server", server.Name, "tool_name", item.Name, "error", err)
				continue
			}
			registered[item.Name] = struct{}{}
			accepted++
			tools = append(tools, &mcpTool{session: session, server: server.Name, info: info})
		}
		slog.Info("MCP 服务工具目录已读取", "agent_run_id", runID, "mcp_server", server.Name,
			"tool_count", len(catalog), "registered_tool_count", accepted)
		if accepted == 0 {
			release()
			continue
		}
		releases = append(releases, release)
	}
	return tools, releaseSessions
}

type mcpHandshake struct {
	session *mcp.Session
	tools   []mcp.Tool
	err     error
}

// openMCPServer 在握手超时内建立会话并读取工具目录，返回释放会话的函数。
// 超时由独立计时器控制，会话 context 在整个 Run 内保持有效，供 SSE 维持挂起的事件流请求。
func openMCPServer(ctx context.Context, server MCPServer) (*mcp.Session, []mcp.Tool, func(), error) {
	sessionCtx, cancelSession := context.WithCancel(ctx)
	done := make(chan mcpHandshake, 1)
	go func() {
		session, err := mcp.Connect(sessionCtx, server.Config)
		if err != nil {
			done <- mcpHandshake{err: err}
			return
		}
		tools, err := session.Tools(sessionCtx)
		if err != nil {
			session.Close()
			done <- mcpHandshake{err: err}
			return
		}
		done <- mcpHandshake{session: session, tools: tools}
	}()
	timer := time.NewTimer(mcpHandshakeTimeout)
	defer timer.Stop()
	select {
	case result := <-done:
		if result.err != nil {
			cancelSession()
			return nil, nil, nil, result.err
		}
		// 已 Detach 的连接只能由 Close 释放，关闭后再结束会话 context。
		return result.session, result.tools, func() {
			if err := result.session.Close(); err != nil {
				slog.Warn("关闭 MCP 会话失败", "mcp_server", server.Name, "error", err)
			}
			cancelSession()
		}, nil
	case <-timer.C:
		cancelSession()
		// 关闭迟到返回的会话。
		go func() {
			if result := <-done; result.session != nil {
				result.session.Close()
			}
		}()
		return nil, nil, nil, errors.New("connect and read tool catalog timed out")
	}
}

// mcpToolInfo 把远程工具目录项转换为模型可见的工具定义。
func mcpToolInfo(item mcp.Tool) (*schema.ToolInfo, error) {
	parameters := &jsonschema.Schema{Type: "object"}
	if len(item.InputSchema) > 0 {
		if err := json.Unmarshal(item.InputSchema, parameters); err != nil {
			return nil, err
		}
	}
	return &schema.ToolInfo{
		Name: item.Name, Desc: item.Description, ParamsOneOf: schema.NewParamsOneOfByJSONSchema(parameters),
	}, nil
}

// mcpTool 把远程 MCP 工具暴露为 Eino 可调用工具。
type mcpTool struct {
	session *mcp.Session
	server  string
	info    *schema.ToolInfo
}

// Info 返回远程工具目录中的名称、描述和参数定义。
func (t *mcpTool) Info(context.Context) (*schema.ToolInfo, error) { return t.info, nil }

// InvokableRun 调用远程工具并返回文本结果。
func (t *mcpTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	arguments := json.RawMessage(argumentsInJSON)
	if !json.Valid(arguments) {
		return "参数不是合法 JSON，请重新提交。", nil
	}
	result, err := t.session.Call(ctx, t.info.Name, arguments)
	if err != nil {
		return "", fmt.Errorf("call tool %q on MCP server %q: %w", t.info.Name, t.server, err)
	}
	return result, nil
}
