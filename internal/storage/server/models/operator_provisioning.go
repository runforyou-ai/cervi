//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// OperatorProvisioning 表示运营开通请求的幂等记录。
type OperatorProvisioning struct {
	bun.BaseModel `bun:"table:operator_provisionings,alias:op"`

	ProvisioningID string    `bun:"provisioning_id,pk"`
	RequestDigest  string    `bun:"request_digest"`
	OrganizationID string    `bun:"organization_id"`
	InitialUserID  string    `bun:"initial_user_id"`
	CreatedAt      time.Time `bun:"created_at"`
	UpdatedAt      time.Time `bun:"updated_at,nullzero,default:now()"`
}
