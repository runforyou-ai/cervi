//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// CustomerCopilotThread 表示客户会话中 Copilot 线程的所属客户会话、AI 员工和创建人。
type CustomerCopilotThread struct {
	bun.BaseModel          `bun:"table:customer_copilot_threads,alias:cct"`
	ConversationID         string    `bun:"conversation_id,pk"`
	CreatedAt              time.Time `bun:"created_at"`
	UpdatedAt              time.Time `bun:"updated_at"`
	OrganizationID         string    `bun:"organization_id"`
	CustomerConversationID string    `bun:"customer_conversation_id"`
	AgentIdentityID        string    `bun:"agent_identity_id"`
	CreatedByIdentityID    string    `bun:"created_by_identity_id"`
}
