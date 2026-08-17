//go:build server

package models

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

// Setting 表示企业的一项配置。
type Setting struct {
	bun.BaseModel `bun:"table:settings,alias:s"`

	OrganizationID string          `bun:"organization_id,pk"`
	Key            string          `bun:"key,pk"`
	Value          json.RawMessage `bun:"value,type:jsonb"`
	CreatedAt      time.Time       `bun:"created_at"`
	UpdatedAt      time.Time       `bun:"updated_at"`
}
