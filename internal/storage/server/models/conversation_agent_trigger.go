//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// ConversationAgentTrigger 表示一条等待 Agent 消费的用户消息。
type ConversationAgentTrigger struct {
	bun.BaseModel `bun:"table:conversation_agent_triggers,alias:cat"`

	ID               string    `bun:"id,pk"`
	OrganizationID   string    `bun:"organization_id"`
	ConversationID   string    `bun:"conversation_id"`
	AgentIdentityID  string    `bun:"agent_identity_id"`
	TriggerType      string    `bun:"trigger_type"`
	ServiceSessionID *string   `bun:"service_session_id"`
	TriggerSeq       int64     `bun:"trigger_seq"`
	TriggerMessageID string    `bun:"trigger_message_id"`
	AgentRunID       *string   `bun:"agent_run_id"`
	CreatedAt        time.Time `bun:"created_at"`
}
