//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// DeviceWorkspace 表示 PostgreSQL 中设备上的 Agent 工作区。
type DeviceWorkspace struct {
	bun.BaseModel `bun:"table:device_workspaces,alias:dw"`

	ID             string     `bun:"id,pk"`
	OrganizationID string     `bun:"organization_id"`
	DeviceID       string     `bun:"device_id"`
	Label          string     `bun:"label"`
	LastUsedAt     *time.Time `bun:"last_used_at"`
	CreatedAt      time.Time  `bun:"created_at"`
	UpdatedAt      time.Time  `bun:"updated_at"`
}
