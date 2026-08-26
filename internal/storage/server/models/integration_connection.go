//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// IntegrationConnectionConfiguration 表示连接器的通用认证配置。
// json 标签用于 jsonb 列的存储编码，不承担 API 契约。
type IntegrationConnectionConfiguration struct {
	APIURL string `json:"apiUrl"`
	APIKey string `json:"apiKey"`
}

// IntegrationConnection 表示 PostgreSQL 中的外部系统连接器。
type IntegrationConnection struct {
	bun.BaseModel `bun:"table:integration_connections,alias:ic"`

	ID             string                             `bun:"id,pk"`
	OrganizationID string                             `bun:"organization_id"`
	Type           string                             `bun:"connector_type"`
	Name           string                             `bun:"name"`
	Description    string                             `bun:"description"`
	Configuration  IntegrationConnectionConfiguration `bun:"configuration,type:jsonb"`
	Status         string                             `bun:"status"`
	LastTestedAt   *time.Time                         `bun:"last_tested_at"`
	CreatedAt      time.Time                          `bun:"created_at"`
	UpdatedAt      time.Time                          `bun:"updated_at"`
}
