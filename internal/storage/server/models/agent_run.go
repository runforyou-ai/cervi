//go:build server

package models

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

// AgentRun 表示一次可吸收多条用户输入的 Agent 业务运行。
type AgentRun struct {
	bun.BaseModel `bun:"table:agent_runs,alias:agr"`

	ID                string          `bun:"id,pk"`
	OrganizationID    string          `bun:"organization_id"`
	ConversationID    string          `bun:"conversation_id"`
	AgentIdentityID   string          `bun:"agent_identity_id"`
	AgentRevisionID   string          `bun:"agent_revision_id"`
	TriggerType       string          `bun:"trigger_type"`
	ServiceSessionID  *string         `bun:"service_session_id"`
	Status            string          `bun:"status"`
	TriggerStartSeq   int64           `bun:"trigger_start_seq"`
	TriggerEndSeq     *int64          `bun:"trigger_end_seq"`
	ResponseMessageID *string         `bun:"response_message_id"`
	Usage             json.RawMessage `bun:"usage,type:jsonb"`
	LastError         *string         `bun:"last_error"`
	ErrorCode         *string         `bun:"error_code"`
	StartedAt         *time.Time      `bun:"started_at"`
	CompletedAt       *time.Time      `bun:"completed_at"`
	CreatedAt         time.Time       `bun:"created_at"`
	UpdatedAt         time.Time       `bun:"updated_at"`
}
