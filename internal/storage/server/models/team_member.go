//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// TeamMember 表示 PostgreSQL 中的团队成员关系。
type TeamMember struct {
	bun.BaseModel `bun:"table:team_members,alias:tm"`

	ID              string    `bun:"id,pk" json:"id"`
	OrganizationID  string    `bun:"organization_id" json:"organizationId"`
	TeamID          string    `bun:"team_id" json:"teamId"`
	MemberID        string    `bun:"member_id" json:"memberId"`
	CreatedByUserID string    `bun:"created_by_user_id" json:"createdByUserId"`
	CreatedAt       time.Time `bun:"created_at" json:"createdAt"`
}
