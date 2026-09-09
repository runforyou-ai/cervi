//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// AgentMCPServer 表示 AI 员工当前绑定的企业 MCP 服务。
type AgentMCPServer struct {
	bun.BaseModel `bun:"table:agent_mcp_servers,alias:ams"`

	OrganizationID string    `bun:"organization_id,pk"`
	AgentID        string    `bun:"agent_id,pk"`
	MCPServerID    string    `bun:"mcp_server_id,pk"`
	CreatedAt      time.Time `bun:"created_at"`
}
