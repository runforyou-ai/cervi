//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// IntegrationConnectionConfiguration 表示连接器的通用认证配置。
type IntegrationConnectionConfiguration struct {
	APIURL string `json:"apiUrl"`
	APIKey string `json:"apiKey"`
}

// IntegrationConnection 表示 PostgreSQL 中的外部系统连接器。
type IntegrationConnection struct {
	bun.BaseModel `bun:"table:integration_connections,alias:ic"`

	ID             string                             `bun:"id,pk" json:"id"`
	OrganizationID string                             `bun:"organization_id" json:"organizationId"`
	Type           string                             `bun:"connector_type" json:"type"`
	Name           string                             `bun:"name" json:"name"`
	Description    string                             `bun:"description" json:"description"`
	Configuration  IntegrationConnectionConfiguration `bun:"configuration,type:jsonb" json:"configuration"`
	Status         string                             `bun:"status" json:"status"`
	LastTestedAt   *time.Time                         `bun:"last_tested_at" json:"lastTestedAt"`
	CreatedAt      time.Time                          `bun:"created_at" json:"createdAt"`
	UpdatedAt      time.Time                          `bun:"updated_at" json:"updatedAt"`
}
