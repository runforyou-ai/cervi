package appservice

import (
	"github.com/runforyou-ai/cervi/internal/domain"
	"time"
)

// MCPServerType 定义 MCP 服务的传输类型。
type MCPServerType string

const (
	MCPServerTypeSSE            MCPServerType = MCPServerType(domain.MCPServerTypeSSE)
	MCPServerTypeStreamableHTTP MCPServerType = MCPServerType(domain.MCPServerTypeStreamableHTTP)
)

// MCPServer 定义企业配置的 MCP 服务。
type MCPServer struct {
	ID                 string        `json:"id"`
	Name               string        `json:"name"`
	URL                string        `json:"url"`
	ServerType         MCPServerType `json:"serverType"`
	AuthorizationToken string        `json:"authorizationToken"`
	Tools              []MCPTool     `json:"tools"`
	ToolsUpdatedAt     *time.Time    `json:"toolsUpdatedAt"`
	ToolsUpdating      bool          `json:"toolsUpdating"`
	ToolsError         string        `json:"toolsError"`
	CreatedAt          time.Time     `json:"createdAt"`
	UpdatedAt          time.Time     `json:"updatedAt"`
}

// MCPServerInput 定义 MCP 服务可编辑字段。
type MCPServerInput struct {
	Name               string        `json:"name"`
	URL                string        `json:"url"`
	ServerType         MCPServerType `json:"serverType"`
	AuthorizationToken string        `json:"authorizationToken"`
}

// MCPServerList 定义企业 MCP 服务列表。
type MCPServerList struct {
	MCPServers []MCPServer `json:"mcpServers"`
}

// MCPTool 定义工具目录的展示信息。
type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// MCPServerConnectionInput 定义 MCP 连接测试字段。
type MCPServerConnectionInput struct {
	URL                string        `json:"url"`
	ServerType         MCPServerType `json:"serverType"`
	AuthorizationToken string        `json:"authorizationToken"`
}
