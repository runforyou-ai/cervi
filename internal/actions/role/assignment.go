//go:build server

package role

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"uuid"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ValidateAssignment 校验并锁定企业身份可以使用的角色。
func ValidateAssignment(ctx context.Context, db bun.IDB, organizationID, roleID string, identityType domain.OrganizationIdentityType) (*servermodels.Role, error) {
	if !common.ValidUUID(roleID) {
		return nil, ErrAssignmentInvalid
	}
	role := &servermodels.Role{}
	err := db.NewSelect().Model(role).
		Column("id", "kind", "name").
		Where("organization_id = ?", organizationID).
		Where("id = ?", roleID).
		For("KEY SHARE").
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAssignmentInvalid
	}
	if err != nil {
		return nil, err
	}
	if identityType == domain.OrganizationIdentityTypeAgent && domain.RoleKind(role.Kind) == domain.RoleKindAdmin {
		return nil, ErrAgentAdministrator
	}
	return role, nil
}

// LockAdministratorRole 锁定管理员角色以串行维护有效管理员数量。
func LockAdministratorRole(ctx context.Context, db bun.IDB, organizationID string) (string, error) {
	role := &servermodels.Role{}
	err := db.NewSelect().Model(role).
		Column("id").
		Where("organization_id = ?", organizationID).
		Where("kind = ?", domain.RoleKindAdmin).
		For("UPDATE").
		Scan(ctx)
	if err != nil {
		return "", err
	}
	return role.ID, nil
}

// EnsureActiveAdministratorRemains 校验企业仍有账号正常的真人管理员。
func EnsureActiveAdministratorRemains(ctx context.Context, db bun.IDB, organizationID, administratorRoleID string) error {
	count, err := db.NewSelect().TableExpr("users AS u").
		Join("JOIN organization_identities AS oi ON oi.id = u.identity_id AND oi.organization_id = u.organization_id AND oi.type = ?", domain.OrganizationIdentityTypeUser).
		Where("u.organization_id = ?", organizationID).
		Where("oi.role_id = ?", administratorRoleID).
		Where("u.status = ?", domain.UserStatusActive).
		Count(ctx)
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrLastActiveAdministrator
	}
	return nil
}

// ServiceSessionHandoff 在角色调整事务中把失去客服角色的 AI 员工负责的开放客服周期交给人工，并在提交后中断被取消的模型调用。
type ServiceSessionHandoff interface {
	HandOffAgentServiceSessions(ctx context.Context, db bun.IDB, organizationID, agentIdentityID, operationID string) ([]string, error)
	CancelRunContexts([]string)
}

// UpdateAssignmentsAction 批量调整真人和 AI 员工的企业角色。
type UpdateAssignmentsAction struct {
	db      *bun.DB
	handoff ServiceSessionHandoff
}

// NewUpdateAssignmentsAction 创建企业身份角色批量调整操作。
func NewUpdateAssignmentsAction(db *bun.DB, handoff ServiceSessionHandoff) *UpdateAssignmentsAction {
	return &UpdateAssignmentsAction{db: db, handoff: handoff}
}

// Execute 校验企业身份和角色后一次性保存全部调整；AI 员工由客服改为其他角色时，把其负责的开放客服周期交给人工。
func (a *UpdateAssignmentsAction) Execute(ctx context.Context, identity *servermodels.Identity, changes []AssignmentInput) error {
	identityIDs := make([]string, 0, len(changes))
	roleIDs := make([]string, 0, len(changes))
	seenIdentities := make(map[string]struct{}, len(changes))
	seenRoles := make(map[string]struct{}, len(changes))
	for index := range changes {
		change := &changes[index]
		var identityValid, roleValid bool
		change.IdentityID, identityValid = common.NormalizeUUID(change.IdentityID)
		change.RoleID, roleValid = common.NormalizeUUID(change.RoleID)
		if !identityValid || !roleValid {
			return ErrAssignmentInvalid
		}
		if _, exists := seenIdentities[change.IdentityID]; exists {
			return ErrAssignmentInvalid
		}
		seenIdentities[change.IdentityID] = struct{}{}
		identityIDs = append(identityIDs, change.IdentityID)
		if _, exists := seenRoles[change.RoleID]; !exists {
			seenRoles[change.RoleID] = struct{}{}
			roleIDs = append(roleIDs, change.RoleID)
		}
	}

	var cancelledRunIDs []string
	err := realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		if len(changes) == 0 {
			return nil
		}
		administratorRoleID, err := LockAdministratorRole(ctx, tx, identity.Organization.ID)
		if err != nil {
			return err
		}
		var roles []servermodels.Role
		if err := tx.NewSelect().Model(&roles).
			Column("id", "kind").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id IN (?)", bun.In(roleIDs)).
			For("KEY SHARE").
			Scan(ctx); err != nil {
			return err
		}
		if len(roles) != len(roleIDs) {
			return ErrAssignmentInvalid
		}
		roleKinds := make(map[string]domain.RoleKind, len(roles))
		for _, role := range roles {
			roleKinds[role.ID] = domain.RoleKind(role.Kind)
		}
		// 先锁定真人账号，保持用户账号先于企业身份的锁序。
		if _, err := tx.NewSelect().Model((*servermodels.User)(nil)).Column("id").
			Where("organization_id = ? AND identity_id IN (?)", identity.Organization.ID, bun.In(identityIDs)).
			OrderExpr("id").For("NO KEY UPDATE").Exec(ctx); err != nil {
			return err
		}
		// 多个企业身份按编号顺序加锁，同时读取调整前的角色类型。
		var identities []struct {
			ID       string          `bun:"id"`
			Type     string          `bun:"type"`
			RoleKind domain.RoleKind `bun:"role_kind"`
		}
		if err := tx.NewSelect().TableExpr("organization_identities AS oi").
			ColumnExpr("oi.id, oi.type, r.kind AS role_kind").
			Join("JOIN roles AS r ON r.id = oi.role_id AND r.organization_id = oi.organization_id").
			Where("oi.organization_id = ?", identity.Organization.ID).
			Where("oi.id IN (?)", bun.In(identityIDs)).
			OrderExpr("oi.id").
			For("UPDATE OF oi").
			Scan(ctx, &identities); err != nil {
			return err
		}
		if len(identities) != len(identityIDs) {
			return ErrAssignmentInvalid
		}
		identityTypes := make(map[string]domain.OrganizationIdentityType, len(identities))
		for _, stored := range identities {
			identityTypes[stored.ID] = domain.OrganizationIdentityType(stored.Type)
		}
		for _, change := range changes {
			if identityTypes[change.IdentityID] == domain.OrganizationIdentityTypeAgent && roleKinds[change.RoleID] == domain.RoleKindAdmin {
				return ErrAgentAdministrator
			}
			query := tx.NewUpdate().Model((*servermodels.OrganizationIdentity)(nil)).
				Set("role_id = ?", change.RoleID).
				Set("updated_at = now()")
			if identityTypes[change.IdentityID] == domain.OrganizationIdentityTypeUser {
				_, err = identityaction.UpdateUserIdentity(ctx, tx, identity.Organization.ID, change.IdentityID, query)
			} else {
				_, err = query.Where("organization_id = ? AND id = ?", identity.Organization.ID, change.IdentityID).Exec(ctx)
			}
			if err != nil {
				return err
			}
		}
		// 身份均已锁定后，按编号顺序交接失去客服角色的 AI 员工负责的开放周期。
		operationID := uuid.NewV7().String()
		newKinds := make(map[string]domain.RoleKind, len(changes))
		for _, change := range changes {
			newKinds[change.IdentityID] = roleKinds[change.RoleID]
		}
		for _, stored := range identities {
			if domain.OrganizationIdentityType(stored.Type) != domain.OrganizationIdentityTypeAgent ||
				stored.RoleKind != domain.RoleKindCustomerService || newKinds[stored.ID] == domain.RoleKindCustomerService {
				continue
			}
			runIDs, err := a.handoff.HandOffAgentServiceSessions(ctx, tx, identity.Organization.ID, stored.ID, operationID)
			if err != nil {
				return err
			}
			cancelledRunIDs = append(cancelledRunIDs, runIDs...)
		}
		return EnsureActiveAdministratorRemains(ctx, tx, identity.Organization.ID, administratorRoleID)
	})
	if err != nil {
		return fmt.Errorf("update role assignments: %w", err)
	}
	a.handoff.CancelRunContexts(cancelledRunIDs)
	return nil
}
