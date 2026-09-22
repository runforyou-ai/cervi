//go:build server

package identity

import (
	"context"
	"errors"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const activeAgentRevisionSchemaVersion = 1

// ErrCustomerHandlingRequired 表示当前身份未开启接待客户。
var ErrCustomerHandlingRequired = errors.New("customer handling identity required")

// LockActiveCustomerHandlingUser 锁定当前真人身份的有效账号，并校验其已开启接待客户；未开启时返回 ErrCustomerHandlingRequired。
func LockActiveCustomerHandlingUser(ctx context.Context, tx bun.Tx, identity *servermodels.Identity) error {
	if err := LockActiveUser(ctx, tx, identity); err != nil {
		return err
	}
	var handlesCustomers bool
	if err := tx.NewSelect().Model((*servermodels.OrganizationIdentity)(nil)).
		Column("oi.handles_customers").
		Where("oi.organization_id = ? AND oi.id = ?", identity.Organization.ID, identity.OrganizationIdentity.ID).
		Scan(ctx, &handlesCustomers); err != nil {
		return err
	}
	if !handlesCustomers {
		return ErrCustomerHandlingRequired
	}
	return nil
}

// ListActiveCustomerHandlingIdentities 返回有效的接待身份。
func ListActiveCustomerHandlingIdentities(ctx context.Context, db bun.IDB, organizationID string) ([]servermodels.OrganizationIdentity, error) {
	identities := make([]servermodels.OrganizationIdentity, 0)
	err := customerHandlingIdentityQuery(db, &identities, organizationID).
		OrderExpr("lower(oi.display_name) ASC, oi.id ASC").
		Scan(ctx)
	return identities, err
}

// LoadActiveCustomerHandlingIdentity 返回指定的有效接待身份。
func LoadActiveCustomerHandlingIdentity(ctx context.Context, db bun.IDB, organizationID, identityID string) (*servermodels.OrganizationIdentity, error) {
	identity := &servermodels.OrganizationIdentity{}
	err := customerHandlingIdentityQuery(db, identity, organizationID).
		Where("oi.id = ?", identityID).
		Scan(ctx)
	return identity, err
}

// LockActiveCustomerHandlingIdentity 对指定身份取 FOR KEY SHARE，再以锁后的语句快照返回有效接待身份，锁等待期间提交的停用或关闭接待随之生效。
func LockActiveCustomerHandlingIdentity(ctx context.Context, db bun.IDB, organizationID, identityID string) (*servermodels.OrganizationIdentity, error) {
	var lockedID string
	if err := db.NewSelect().Model((*servermodels.OrganizationIdentity)(nil)).
		Column("oi.id").
		Where("oi.organization_id = ? AND oi.id = ?", organizationID, identityID).
		For("KEY SHARE").
		Scan(ctx, &lockedID); err != nil {
		return &servermodels.OrganizationIdentity{}, err
	}
	return LoadActiveCustomerHandlingIdentity(ctx, db, organizationID, identityID)
}

// ApplyCustomerHandlingConditions 给以 oi 为别名的企业身份查询追加有效接待身份条件：开启接待且账号有效，AI 员工另需托管执行与当前 Revision Schema 版本。
func ApplyCustomerHandlingConditions(query *bun.SelectQuery) *bun.SelectQuery {
	return query.
		Where("oi.handles_customers").
		Where(`((oi.type = ? AND EXISTS (
				SELECT 1 FROM users AS hu
				WHERE hu.identity_id = oi.id AND hu.organization_id = oi.organization_id AND hu.status = ?))
			OR (oi.type = ? AND EXISTS (
				SELECT 1 FROM agents AS ha
				JOIN agent_revisions AS har ON har.id = ha.active_revision_id AND har.agent_id = ha.id AND har.organization_id = ha.organization_id
				WHERE ha.identity_id = oi.id AND ha.organization_id = oi.organization_id AND ha.status = ?
					AND har.execution_mode = ? AND har.schema_version = ?)))`,
			domain.OrganizationIdentityTypeUser, domain.UserStatusActive,
			domain.OrganizationIdentityTypeAgent, domain.UserStatusActive,
			domain.AgentExecutionModeManaged, activeAgentRevisionSchemaVersion,
		)
}

// customerHandlingIdentityQuery 构造统一的有效接待身份查询。
func customerHandlingIdentityQuery(db bun.IDB, model any, organizationID string) *bun.SelectQuery {
	return ApplyCustomerHandlingConditions(db.NewSelect().Model(model).
		Column("oi.id", "oi.organization_id", "oi.type", "oi.role_id", "oi.display_name", "oi.avatar_file_id", "oi.handles_customers", "oi.work_status").
		Where("oi.organization_id = ?", organizationID))
}

// TeamCustomerHandlerQuery 构造团队内开启接待真人成员的存在性查询，调用方以 oi 与 tm 别名追加企业和团队条件。
func TeamCustomerHandlerQuery(db bun.IDB) *bun.SelectQuery {
	return ApplyCustomerHandlingConditions(db.NewSelect().
		TableExpr("organization_identities AS oi").ColumnExpr("1").
		Join("JOIN team_members AS tm ON tm.organization_id = oi.organization_id AND tm.identity_id = oi.id").
		Where("oi.type = ?", domain.OrganizationIdentityTypeUser))
}

// TeamHasCustomerHandler 判断团队内是否存在开启接待的有效真人成员。
func TeamHasCustomerHandler(ctx context.Context, db bun.IDB, organizationID, teamID string) (bool, error) {
	return TeamCustomerHandlerQuery(db).
		Where("oi.organization_id = ? AND tm.team_id = ?", organizationID, teamID).
		Exists(ctx)
}
