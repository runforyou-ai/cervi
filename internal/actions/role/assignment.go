//go:build server

package role

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
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

// UpdateAssignmentsAction 批量调整真人和 AI 员工的企业角色。
type UpdateAssignmentsAction struct{ db *bun.DB }

// NewUpdateAssignmentsAction 创建企业身份角色批量调整操作。
func NewUpdateAssignmentsAction(db *bun.DB) *UpdateAssignmentsAction {
	return &UpdateAssignmentsAction{db: db}
}

// Execute 校验企业身份和角色后一次性保存全部调整。
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

	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
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
		var identities []servermodels.OrganizationIdentity
		if err := tx.NewSelect().Model(&identities).
			Column("id", "type").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id IN (?)", bun.In(identityIDs)).
			For("UPDATE").
			Scan(ctx); err != nil {
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
			if _, err := tx.NewUpdate().Model((*servermodels.OrganizationIdentity)(nil)).
				Set("role_id = ?", change.RoleID).
				Set("updated_at = now()").
				Where("organization_id = ?", identity.Organization.ID).
				Where("id = ?", change.IdentityID).
				Exec(ctx); err != nil {
				return err
			}
		}
		return EnsureActiveAdministratorRemains(ctx, tx, identity.Organization.ID, administratorRoleID)
	})
	if err != nil {
		return fmt.Errorf("update role assignments: %w", err)
	}
	return nil
}
