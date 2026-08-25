//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// AgentRevision 表示 PostgreSQL 中不可变的 AI 员工能力配置。
type AgentRevision struct {
	bun.BaseModel `bun:"table:agent_revisions,alias:ar"`

	ID                string    `bun:"id,pk" json:"id"`
	OrganizationID    string    `bun:"organization_id" json:"organizationId"`
	AgentID           string    `bun:"agent_id" json:"agentId"`
	ProviderID        string    `bun:"provider_id" json:"providerId"`
	ModelIdentifier   string    `bun:"model_identifier" json:"modelIdentifier"`
	SystemInstruction string    `bun:"system_instruction" json:"systemInstruction"`
	CreatedByUserID   string    `bun:"created_by_user_id" json:"createdByUserId"`
	CreatedAt         time.Time `bun:"created_at" json:"createdAt"`
}
