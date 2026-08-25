//go:build server

package models

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

// AgentRevision 表示 PostgreSQL 中不可变的 AI 员工执行配置。
type AgentRevision struct {
	bun.BaseModel `bun:"table:agent_revisions,alias:ar"`

	ID              string          `bun:"id,pk" json:"id"`
	OrganizationID  string          `bun:"organization_id" json:"organizationId"`
	AgentID         string          `bun:"agent_id" json:"agentId"`
	ExecutionMode   string          `bun:"execution_mode" json:"executionMode"`
	SchemaVersion   int             `bun:"schema_version" json:"schemaVersion"`
	Configuration   json.RawMessage `bun:"configuration,type:jsonb" json:"configuration"`
	CreatedByUserID string          `bun:"created_by_user_id" json:"createdByUserId"`
	CreatedAt       time.Time       `bun:"created_at" json:"createdAt"`
}
