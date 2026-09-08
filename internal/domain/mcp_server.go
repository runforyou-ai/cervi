package domain

// MCPServerType 定义 MCP 服务的传输类型。
type MCPServerType string

const (
	MCPServerTypeSSE            MCPServerType = "sse"
	MCPServerTypeStreamableHTTP MCPServerType = "streamable-http"
)

// MCPTool 定义远程工具目录中的名称和描述。
type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
