//go:build server

package user

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	commonpassword "github.com/runforyou-ai/cervi/internal/common/password"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// CreateUserAction 创建企业成员账号。
type CreateUserAction struct{ db *bun.DB }

// NewCreateUserAction 创建企业成员新增操作。
func NewCreateUserAction(db *bun.DB) *CreateUserAction { return &CreateUserAction{db: db} }

// Execute 校验并创建企业成员及其团队关系。
func (a *CreateUserAction) Execute(ctx context.Context, identity *servermodels.Identity, input CreateInput) (*User, error) {
	input, fields := normalizeCreateInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	passwordHash, err := commonpassword.Hash(input.Password)
	if err != nil {
		return nil, fmt.Errorf("hash member password: %w", err)
	}
	var output *User
	err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		if err := validateRoleID(ctx, tx, identity.Organization.ID, input.RoleID); err != nil {
			return err
		}
		organizationIdentity := &servermodels.OrganizationIdentity{
			OrganizationID: identity.Organization.ID,
			Type:           string(domain.OrganizationIdentityTypeUser),
			RoleID:         input.RoleID,
			DisplayName:    input.DisplayName,
			WorkStatus:     string(domain.WorkStatusWorking),
		}
		_, err := tx.NewInsert().Model(organizationIdentity).
			Column("organization_id", "type", "role_id", "display_name", "work_status").Returning("id").Exec(ctx)
		if err != nil {
			return err
		}
		user := &servermodels.User{
			IdentityID:     organizationIdentity.ID,
			OrganizationID: identity.Organization.ID,
			Email:          input.Email,
			PasswordHash:   passwordHash,
			Status:         string(domain.UserStatusActive),
			Locale:         identity.User.Locale,
			TimeZone:       identity.User.TimeZone,
		}
		_, err = tx.NewInsert().Model(user).
			Column("identity_id", "organization_id", "email", "password_hash", "status", "locale", "time_zone").Returning("id").Exec(ctx)
		if isUniqueViolation(err) {
			return &ValidationError{Fields: map[string]ValidationCode{"email": ValidationEmailDuplicate}}
		}
		if err != nil {
			return err
		}
		if err := replaceUserTeams(ctx, tx, identity, user.IdentityID, input.TeamIDs); err != nil {
			return err
		}
		output, err = loadUser(ctx, tx, identity.Organization.ID, user.ID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return output, nil
}
