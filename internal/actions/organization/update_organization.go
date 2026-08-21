//go:build server

package organization

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateOrganizationAction 修改企业名称。
type UpdateOrganizationAction struct {
	db *bun.DB
}

// NewUpdateOrganizationAction 创建企业名称修改操作。
func NewUpdateOrganizationAction(db *bun.DB) *UpdateOrganizationAction {
	return &UpdateOrganizationAction{db: db}
}

// Execute 校验并修改当前用户所属企业的名称。
func (a *UpdateOrganizationAction) Execute(ctx context.Context, identity *servermodels.Identity, input Input) (*servermodels.Organization, error) {
	input, fields := normalizeInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	if identity == nil ||
		!common.ValidUUID(identity.Organization.ID) ||
		!common.ValidUUID(identity.User.ID) ||
		identity.User.OrganizationID != identity.Organization.ID {
		return nil, common.ErrIdentityInvalid
	}

	organization := &servermodels.Organization{}
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := tx.NewSelect().
			Model(organization).
			Column("id").
			Where("o.id = ?", identity.Organization.ID).
			For("UPDATE").
			Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return common.ErrIdentityInvalid
			}
			return err
		}
		user := &servermodels.User{}
		if err := tx.NewSelect().
			Model(user).
			Column("id").
			Where("u.id = ?", identity.User.ID).
			Where("u.organization_id = ?", identity.Organization.ID).
			For("KEY SHARE").
			Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return common.ErrIdentityInvalid
			}
			return err
		}
		_, err := tx.NewUpdate().
			Model(organization).
			Set("name = ?", input.Name).
			Set("updated_at = now()").
			WherePK().
			Exec(ctx)
		return err
	})
	if errors.Is(err, common.ErrIdentityInvalid) {
		return nil, common.ErrIdentityInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("update organization: %w", err)
	}
	organization.Name = input.Name
	return organization, nil
}
