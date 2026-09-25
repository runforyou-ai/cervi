package agentruntime

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const (
	// addLocalMCPToolName 是添加本地 MCP 服务的工具名称。
	addLocalMCPToolName = "add_local_mcp_server"
	// removeLocalMCPToolName 是移除本地 MCP 服务的工具名称。
	removeLocalMCPToolName = "remove_local_mcp_server"
)

// localMCPToolNames 是本地 MCP 管理工具，按注册顺序排列。
var localMCPToolNames = []string{addLocalMCPToolName, removeLocalMCPToolName}

const addLocalMCPToolDesc = `为这台电脑添加本地 MCP 服务，这台电脑上主人的所有助理共用。
- 添加前会试启动服务并读取工具目录，失败时返回原因；成功后返回服务提供的工具。
- command 可以直接使用 uvx、npx、uv、python、node，这些由 Cervi 提供，无需另行安装；例如 command 为 uvx、args 为 ["mcp-server-fetch"]。
- name 只能包含字母、数字、下划线与连字符；同名服务会被替换。
- 新添加的服务从下一次运行起可用。`

const removeLocalMCPToolDesc = `移除这台电脑上的本地 MCP 服务。当前已添加：%s。`

// addLocalMCPArgs 是添加本地 MCP 服务的参数。
type addLocalMCPArgs struct {
	Name    string            `json:"name" jsonschema:"description=服务名称"`
	Command string            `json:"command" jsonschema:"description=启动命令"`
	Args    []string          `json:"args,omitempty" jsonschema:"description=启动参数"`
	Env     map[string]string `json:"env,omitempty" jsonschema:"description=服务需要的环境变量"`
}

// removeLocalMCPArgs 是移除本地 MCP 服务的参数。
type removeLocalMCPArgs struct {
	Name string `json:"name" jsonschema:"description=服务名称"`
}

// newLocalMCPTools 按有效配置创建本地 MCP 管理工具，没有列出这些工具时返回空列表。
func newLocalMCPTools(ctx context.Context, request RunRequest) ([]tool.BaseTool, error) {
	if !slices.ContainsFunc(localMCPToolNames, func(name string) bool { return slices.Contains(request.Assignment.Tools, name) }) {
		return nil, nil
	}
	if request.LocalMCP == nil {
		return nil, fmt.Errorf("agent run assignment requires local MCP tools without local MCP management")
	}
	names, err := request.LocalMCP.Names(ctx)
	if err != nil {
		return nil, fmt.Errorf("list local MCP servers: %w", err)
	}
	// 移除工具的说明列出当前已添加的服务。
	current := "无"
	if len(names) > 0 {
		current = strings.Join(names, "、")
	}
	add, err := utils.InferTool(addLocalMCPToolName, addLocalMCPToolDesc, func(ctx context.Context, input addLocalMCPArgs) (string, error) {
		tools, err := request.LocalMCP.Add(ctx, LocalMCPServer{Name: input.Name, Command: input.Command, Args: input.Args, Env: input.Env})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("已添加本地 MCP 服务 %s，提供的工具：%s。下一次运行起可用。", input.Name, strings.Join(tools, "、")), nil
	})
	if err != nil {
		return nil, fmt.Errorf("create add local MCP tool: %w", err)
	}
	remove, err := utils.InferTool(removeLocalMCPToolName, fmt.Sprintf(removeLocalMCPToolDesc, current), func(ctx context.Context, input removeLocalMCPArgs) (string, error) {
		removed, err := request.LocalMCP.Remove(ctx, input.Name)
		if err != nil {
			return "", err
		}
		if !removed {
			return "", fmt.Errorf("没有名为 %s 的本地 MCP 服务", input.Name)
		}
		return "已移除本地 MCP 服务 " + input.Name + "。", nil
	})
	if err != nil {
		return nil, fmt.Errorf("create remove local MCP tool: %w", err)
	}
	return []tool.BaseTool{add, remove}, nil
}
