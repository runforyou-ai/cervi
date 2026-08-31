//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// ConversationAgentState 表示会话 Agent 的输入序号状态。
type ConversationAgentState struct {
	bun.BaseModel `bun:"table:conversation_agent_states,alias:cas"`

	ConversationID  string    `bun:"conversation_id,pk"`
	OrganizationID  string    `bun:"organization_id"`
	AgentIdentityID string    `bun:"agent_identity_id,pk"`
	DesiredSeq      int64     `bun:"desired_seq"`
	ProcessedSeq    int64     `bun:"processed_seq"`
	CreatedAt       time.Time `bun:"created_at"`
	UpdatedAt       time.Time `bun:"updated_at"`
}
