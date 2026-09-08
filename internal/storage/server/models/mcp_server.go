//go:build server

package models

import (
	"github.com/runforyou-ai/cervi/internal/domain"
	"time"

	"github.com/uptrace/bun"
)

// MCPServer 表示 PostgreSQL 中的企业 MCP 服务。
type MCPServer struct {
	bun.BaseModel `bun:"table:mcp_servers,alias:ms"`

	ID                 string               `bun:"id,pk"`
	OrganizationID     string               `bun:"organization_id"`
	Name               string               `bun:"name"`
	URL                string               `bun:"url"`
	ServerType         domain.MCPServerType `bun:"server_type"`
	AuthorizationToken string               `bun:"authorization_token"`
	CreatedAt          time.Time            `bun:"created_at"`
	UpdatedAt          time.Time            `bun:"updated_at"`
}
