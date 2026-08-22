//go:build server

package user

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateUserAction 修改企业成员账号。
type UpdateUserAction struct{ db *bun.DB }

// NewUpdateUserAction 创建企业成员修改操作。
func NewUpdateUserAction(db *bun.DB) *UpdateUserAction { return &UpdateUserAction{db: db} }

// Execute 修改企业成员资料、角色和所属团队。
func (a *UpdateUserAction) Execute(ctx context.Context, identity *servermodels.Identity, userID string, input UpdateInput) (*DirectoryUser, error) {
	input, fields := normalizeUpdateInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	if !common.ValidUUID(userID) {
		return nil, ErrNotFound
	}
	var output *DirectoryUser
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := validateIdentity(ctx, tx, identity); err != nil {
			return err
		}
		administratorRoleID, err := lockAdministratorRole(ctx, tx, identity.Organization.ID)
		if err != nil {
			return err
		}
		if err := validateRoleID(ctx, tx, identity.Organization.ID, input.RoleID); err != nil {
			return err
		}
		result, err := tx.NewUpdate().Model((*servermodels.User)(nil)).
			Set("display_name = ?", input.DisplayName).
			Set("email = ?", input.Email).
			Set("role_id = ?", input.RoleID).
			Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", userID).
			Exec(ctx)
		if isUniqueViolation(err) {
			return &ValidationError{Fields: map[string]ValidationCode{"email": ValidationEmailDuplicate}}
		}
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return ErrNotFound
		}
		if err := ensureActiveAdministratorRemains(ctx, tx, identity.Organization.ID, administratorRoleID); err != nil {
			return err
		}
		if err := replaceUserTeams(ctx, tx, identity, userID, input.TeamIDs); err != nil {
			return err
		}
		output, err = loadDirectoryUser(ctx, tx, identity.Organization.ID, userID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return output, nil
}
