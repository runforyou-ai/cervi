//go:build server

package models

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

// AIProvider 表示 PostgreSQL 中的模型服务供应商。
type AIProvider struct {
	bun.BaseModel `bun:"table:ai_providers,alias:aip"`

	ID             string    `bun:"id,pk" json:"id"`
	OrganizationID string    `bun:"organization_id" json:"organizationId"`
	Brand          string    `bun:"brand" json:"brand"`
	Name           string    `bun:"name" json:"name"`
	APIKey         string    `bun:"api_key" json:"apiKey"`
	APIURL         string    `bun:"api_url" json:"apiUrl"`
	CreatedAt      time.Time `bun:"created_at" json:"createdAt"`
	UpdatedAt      time.Time `bun:"updated_at" json:"updatedAt"`
}

// AIProviderModel 表示 PostgreSQL 中供应商模型目录项。
type AIProviderModel struct {
	bun.BaseModel `bun:"table:ai_provider_models,alias:aipm"`

	ProviderID      string          `bun:"provider_id,pk" json:"providerId"`
	OrganizationID  string          `bun:"organization_id" json:"organizationId"`
	Identifier      string          `bun:"identifier,pk" json:"identifier"`
	Name            string          `bun:"name" json:"name"`
	Type            string          `bun:"model_type" json:"type"`
	InputModalities json.RawMessage `bun:"input_modalities,type:jsonb" json:"inputModalities"`
	ContextWindow   int64           `bun:"context_window" json:"contextWindow"`
	MaxOutputTokens int64           `bun:"max_output_tokens" json:"maxOutputTokens"`
	CreatedAt       time.Time       `bun:"created_at" json:"createdAt"`
}
