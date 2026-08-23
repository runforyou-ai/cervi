//go:build server

// Package identity 提供 Action 层共用的当前身份校验。
package identity

import (
	"context"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// Validate 校验当前企业用户账号仍可用。
func Validate(ctx context.Context, db bun.IDB, identity *servermodels.Identity) error {
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
	active, err := db.NewSelect().
		Model((*servermodels.User)(nil)).
		Where("id = ?", identity.User.ID).
		Where("identity_id = ?", identity.User.IdentityID).
		Where("organization_id = ?", identity.Organization.ID).
		Where("status = ?", domain.UserStatusActive).
		Exists(ctx)
	if err != nil {
		return err
	}
	if !active {
		return common.ErrIdentityInvalid
	}
	return nil
}
