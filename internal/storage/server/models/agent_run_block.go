//go:build server

package models

import (
	"encoding/json"

	"github.com/uptrace/bun"
)

// AgentRunBlock 保存成功运行的一个完整中间内容块。
type AgentRunBlock struct {
	bun.BaseModel  `bun:"table:agent_run_blocks,alias:arb"`
	ID             string          `bun:"id,pk"`
	OrganizationID string          `bun:"organization_id"`
	AgentRunID     string          `bun:"agent_run_id"`
	Position       int64           `bun:"position"`
	ModelCallID    string          `bun:"model_call_id"`
	Kind           string          `bun:"kind"`
	Payload        json.RawMessage `bun:"payload,type:jsonb"`
}
