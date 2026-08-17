//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// ContactMethod 表示外部联系人的邮箱或电话。
type ContactMethod struct {
	bun.BaseModel `bun:"table:contact_methods,alias:cm"`

	ID              string    `bun:"id,pk" json:"id"`
	OrganizationID  string    `bun:"organization_id" json:"organizationId"`
	ContactID       string    `bun:"contact_id" json:"contactId"`
	Type            string    `bun:"type" json:"type"`
	Value           string    `bun:"value" json:"value"`
	NormalizedValue string    `bun:"normalized_value" json:"-"`
	Label           *string   `bun:"label" json:"label"`
	IsPrimary       bool      `bun:"is_primary" json:"isPrimary"`
	CreatedAt       time.Time `bun:"created_at" json:"createdAt"`
	UpdatedAt       time.Time `bun:"updated_at" json:"updatedAt"`
}
