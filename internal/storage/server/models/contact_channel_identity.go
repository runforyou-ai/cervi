//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// ContactChannelIdentity 表示联系人在消息渠道中的身份。
type ContactChannelIdentity struct {
	bun.BaseModel `bun:"table:contact_channel_identities,alias:cci"`

	ID             string     `bun:"id,pk" json:"id"`
	OrganizationID string     `bun:"organization_id" json:"organizationId"`
	ContactID      string     `bun:"contact_id" json:"contactId"`
	ChannelID      string     `bun:"channel_id" json:"channelId"`
	ExternalID     string     `bun:"external_id" json:"externalId"`
	DisplayName    *string    `bun:"display_name" json:"displayName"`
	CreatedAt      time.Time  `bun:"created_at" json:"createdAt"`
	UpdatedAt      time.Time  `bun:"updated_at" json:"updatedAt"`
	LastSeenAt     *time.Time `bun:"last_seen_at" json:"lastSeenAt"`
}
