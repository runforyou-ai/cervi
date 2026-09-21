//go:build !server && !ios && !android

package models

import "github.com/uptrace/bun"

// AgentWorkspace 表示本机 Agent 工作区在企业服务器上的编号与本地真实路径。
type AgentWorkspace struct {
	bun.BaseModel `bun:"table:agent_workspaces,alias:agent_workspace"`

	WorkspaceID    string `bun:"workspace_id,pk"`
	ServerURL      string `bun:"server_url"`
	OrganizationID string `bun:"organization_id"`
	Path           string `bun:"path"`
	CreatedAt      string `bun:"created_at"`
}
