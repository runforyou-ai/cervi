//go:build server

// Package identity 提供 Action 层共用的身份查询和当前用户写入守卫。
package identity

import (
	"context"
	"database/sql"
	"errors"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// LockActiveUser 校验当前身份并锁定有效用户账号，供写事务复用。
func LockActiveUser(ctx context.Context, tx bun.Tx, identity *servermodels.Identity) error {
	if identity == nil ||
		!common.ValidUUID(identity.Organization.ID) ||
		!common.ValidUUID(identity.OrganizationIdentity.ID) ||
		!common.ValidUUID(identity.User.ID) ||
		!common.ValidUUID(identity.User.IdentityID) ||
		identity.OrganizationIdentity.ID != identity.User.IdentityID ||
		identity.OrganizationIdentity.OrganizationID != identity.Organization.ID ||
		identity.OrganizationIdentity.Type != string(domain.OrganizationIdentityTypeUser) ||
		identity.User.OrganizationID != identity.Organization.ID {
		return common.ErrIdentityInvalid
	}
	user := &servermodels.User{}
	err := tx.NewSelect().
		Model(user).
		Column("id").
		Where("id = ?", identity.User.ID).
		Where("identity_id = ?", identity.User.IdentityID).
		Where("organization_id = ?", identity.Organization.ID).
		Where("status = ?", domain.UserStatusActive).
		For("NO KEY UPDATE").
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return common.ErrIdentityInvalid
	}
	if err != nil {
		return err
	}
	return nil
}
