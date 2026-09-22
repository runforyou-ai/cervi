//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// ExternalIdentity 表示企业用户与官方身份服务账号的绑定。
type ExternalIdentity struct {
	bun.BaseModel `bun:"table:external_identities,alias:ei"`

	ID             string    `bun:"id,pk"`
	OrganizationID string    `bun:"organization_id"`
	UserID         string    `bun:"user_id"`
	Issuer         string    `bun:"issuer"`
	Subject        string    `bun:"subject"`
	CreatedAt      time.Time `bun:"created_at"`
	UpdatedAt      time.Time `bun:"updated_at"`
}
