//go:build server

package team

import (
	"context"
	"database/sql"
	"errors"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/uptrace/bun"
)

// withActiveMemberCount 补充团队中账号正常的企业成员和 AI 员工数量。
func withActiveMemberCount(query *bun.SelectQuery) *bun.SelectQuery {
	return query.ColumnExpr(`(
		SELECT count(*)
		FROM team_members AS tm
		JOIN organization_identities AS oi ON oi.id = tm.identity_id AND oi.organization_id = tm.organization_id
		LEFT JOIN users AS u ON u.identity_id = oi.id AND u.organization_id = oi.organization_id
		LEFT JOIN agents AS a ON a.identity_id = oi.id AND a.organization_id = oi.organization_id
		WHERE tm.organization_id = t.organization_id
			AND tm.team_id = t.id
			AND ((oi.type = ? AND u.status = ?) OR (oi.type = ? AND a.status = ?))
	) AS member_count`, domain.OrganizationIdentityTypeUser, domain.UserStatusActive, domain.OrganizationIdentityTypeAgent, domain.UserStatusActive)
}

// loadTeam 读取当前企业中的团队。
func loadTeam(ctx context.Context, db bun.IDB, organizationID, teamID string) (*TeamRecord, error) {
	if !common.ValidUUID(teamID) {
		return nil, ErrNotFound
	}
	record := &TeamRecord{}
	query := db.NewSelect().
		TableExpr("teams AS t").
		ColumnExpr("t.id::text AS id").
		Column("name", "description", "created_at", "updated_at")
	err := withActiveMemberCount(query).
		Where("t.organization_id = ?", organizationID).
		Where("t.id = ?", teamID).
		Scan(ctx, record)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return record, err
}
