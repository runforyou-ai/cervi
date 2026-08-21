//go:build server

package team

import (
	"context"
	"database/sql"
	"errors"

	"github.com/runforyou-ai/cervi/internal/common"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// validateIdentity 校验当前用户仍是企业的有效成员。
func validateIdentity(ctx context.Context, db bun.IDB, identity *servermodels.Identity) error {
	if identity == nil || !common.ValidUUID(identity.Organization.ID) || !common.ValidUUID(identity.User.ID) || identity.User.OrganizationID != identity.Organization.ID {
		return common.ErrIdentityInvalid
	}
	exists, err := db.NewSelect().
		Model((*servermodels.User)(nil)).
		Where("organization_id = ?", identity.Organization.ID).
		Where("id = ?", identity.User.ID).
		Where("status = 'active'").
		Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return common.ErrIdentityInvalid
	}
	return nil
}

// loadTeam 读取当前企业中的团队。
func loadTeam(ctx context.Context, db bun.IDB, organizationID, teamID string) (*TeamRecord, error) {
	if !common.ValidUUID(teamID) {
		return nil, ErrNotFound
	}
	record := &TeamRecord{}
	err := db.NewSelect().
		TableExpr("teams AS t").
		ColumnExpr("t.id::text AS id").
		Column("name", "description", "created_at", "updated_at").
		ColumnExpr("(SELECT count(*) FROM team_members AS tm WHERE tm.organization_id = t.organization_id AND tm.team_id = t.id) AS member_count").
		Where("t.organization_id = ?", organizationID).
		Where("t.id = ?", teamID).
		Scan(ctx, record)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return record, err
}
