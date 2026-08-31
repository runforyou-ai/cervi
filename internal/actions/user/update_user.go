//go:build server

package user

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

// UpdateUserAction 修改企业成员账号。
type UpdateUserAction struct{ db *bun.DB }

// NewUpdateUserAction 创建企业成员修改操作。
func NewUpdateUserAction(db *bun.DB) *UpdateUserAction { return &UpdateUserAction{db: db} }

// Execute 修改企业成员资料、角色和所属团队。
func (a *UpdateUserAction) Execute(ctx context.Context, identity *servermodels.Identity, userID string, input UpdateInput) (*User, error) {
	input, fields := normalizeUpdateInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	if !common.ValidUUID(userID) {
		return nil, ErrNotFound
	}
	var output *User
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		administratorRoleID, err := lockAdministratorRole(ctx, tx, identity.Organization.ID)
		if err != nil {
			return err
		}
		if err := validateRoleID(ctx, tx, identity.Organization.ID, input.RoleID); err != nil {
			return err
		}
		updatedUser := &servermodels.User{}
		err = tx.NewUpdate().Model(updatedUser).
			Set("email = ?", input.Email).
			Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", userID).
			Returning("identity_id").
			Scan(ctx)
		if isUniqueViolation(err) {
			return &ValidationError{Fields: map[string]ValidationCode{"email": ValidationEmailDuplicate}}
		}
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		_, err = tx.NewUpdate().Model((*servermodels.OrganizationIdentity)(nil)).
			Set("display_name = ?", input.DisplayName).
			Set("role_id = ?", input.RoleID).
			Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", updatedUser.IdentityID).
			Where("type = ?", domain.OrganizationIdentityTypeUser).
			Exec(ctx)
		if err != nil {
			return err
		}
		if err := ensureActiveAdministratorRemains(ctx, tx, identity.Organization.ID, administratorRoleID); err != nil {
			return err
		}
		if err := replaceUserTeams(ctx, tx, identity, updatedUser.IdentityID, input.TeamIDs); err != nil {
			return err
		}
		output, err = loadUser(ctx, tx, identity.Organization.ID, userID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return output, nil
}
