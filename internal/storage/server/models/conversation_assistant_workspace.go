//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// ConversationAssistantWorkspace 表示 PostgreSQL 中主人为助理在会话中指定的本机工作区。
type ConversationAssistantWorkspace struct {
	bun.BaseModel `bun:"table:conversation_assistant_workspaces,alias:caw"`

	ID               string    `bun:"id,pk"`
	OrganizationID   string    `bun:"organization_id"`
	ConversationID   string    `bun:"conversation_id"`
	AgentID          string    `bun:"agent_id"`
	WorkspaceID      string    `bun:"workspace_id"`
	AssignedByUserID string    `bun:"assigned_by_user_id"`
	CreatedAt        time.Time `bun:"created_at"`
	UpdatedAt        time.Time `bun:"updated_at"`
}
