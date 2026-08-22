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

// Validate 校验当前用户仍是当前企业的有效成员。
func Validate(ctx context.Context, db bun.IDB, identity *servermodels.Identity) error {
	if identity == nil ||
		!common.ValidUUID(identity.Organization.ID) ||
		!common.ValidUUID(identity.User.ID) ||
		identity.User.OrganizationID != identity.Organization.ID {
		return common.ErrIdentityInvalid
	}
	active, err := db.NewSelect().
		Model((*servermodels.OrganizationMember)(nil)).
		Where("id = ?", identity.User.ID).
		Where("organization_id = ?", identity.Organization.ID).
		Where("type = ?", domain.MemberIdentityTypeUser).
		Where("status = ?", "active").
		Exists(ctx)
	if err != nil {
		return err
	}
	if !active {
		return common.ErrIdentityInvalid
	}
	return nil
}
