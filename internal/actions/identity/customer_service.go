//go:build server

package identity

import (
	"context"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const activeAgentRevisionSchemaVersion = 1

// ListActiveCustomerServiceIdentities 返回有效的真人和 AI 客服身份。
func ListActiveCustomerServiceIdentities(ctx context.Context, db bun.IDB, organizationID string) ([]servermodels.OrganizationIdentity, error) {
	identities := make([]servermodels.OrganizationIdentity, 0)
	err := activeCustomerServiceIdentityQuery(db, &identities, organizationID).
		OrderExpr("lower(oi.display_name) ASC, oi.id ASC").
		Scan(ctx)
	return identities, err
}

// LoadActiveCustomerServiceIdentity 返回指定的有效客服身份。
func LoadActiveCustomerServiceIdentity(ctx context.Context, db bun.IDB, organizationID, identityID string) (*servermodels.OrganizationIdentity, error) {
	identity := &servermodels.OrganizationIdentity{}
	err := activeCustomerServiceIdentityQuery(db, identity, organizationID).
		Where("oi.id = ?", identityID).
		Scan(ctx)
	return identity, err
}

// LockActiveCustomerServiceIdentity 锁定并返回指定的有效客服身份。
func LockActiveCustomerServiceIdentity(ctx context.Context, db bun.IDB, organizationID, identityID string) (*servermodels.OrganizationIdentity, error) {
	identity := &servermodels.OrganizationIdentity{}
	err := activeCustomerServiceIdentityQuery(db, identity, organizationID).
		Where("oi.id = ?", identityID).
		For("KEY SHARE OF oi").
		Scan(ctx)
	return identity, err
}

// activeCustomerServiceIdentityQuery 构造统一的有效客服身份查询。
func activeCustomerServiceIdentityQuery(db bun.IDB, model any, organizationID string) *bun.SelectQuery {
	return db.NewSelect().Model(model).
		Column("oi.id", "oi.organization_id", "oi.type", "oi.role_id", "oi.display_name", "oi.avatar_file_id").
		Join("JOIN roles AS r ON r.id = oi.role_id AND r.organization_id = oi.organization_id AND r.kind = ?", domain.RoleKindCustomerService).
		Join("LEFT JOIN users AS u ON u.identity_id = oi.id AND u.organization_id = oi.organization_id").
		Join("LEFT JOIN agents AS a ON a.identity_id = oi.id AND a.organization_id = oi.organization_id").
		Join("LEFT JOIN agent_revisions AS ar ON ar.id = a.active_revision_id AND ar.agent_id = a.id AND ar.organization_id = a.organization_id").
		Where("oi.organization_id = ?", organizationID).
		Where("((oi.type = ? AND u.status = ?) OR (oi.type = ? AND a.status = ? AND ar.execution_mode = ? AND ar.schema_version = ?))",
			domain.OrganizationIdentityTypeUser, domain.UserStatusActive,
			domain.OrganizationIdentityTypeAgent, domain.UserStatusActive,
			domain.AgentExecutionModeManaged, activeAgentRevisionSchemaVersion,
		)
}
