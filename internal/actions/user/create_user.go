//go:build server

package user

import (
	"context"
	"fmt"

	commonpassword "github.com/runforyou-ai/cervi/internal/common/password"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// CreateUserAction 创建企业成员账号。
type CreateUserAction struct{ db *bun.DB }

// NewCreateUserAction 创建企业成员新增操作。
func NewCreateUserAction(db *bun.DB) *CreateUserAction { return &CreateUserAction{db: db} }

// Execute 校验并创建企业成员及其团队关系。
func (a *CreateUserAction) Execute(ctx context.Context, identity *servermodels.Identity, input CreateInput) (*DirectoryUser, error) {
	input, fields := normalizeCreateInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	passwordHash, err := commonpassword.Hash(input.Password)
	if err != nil {
		return nil, fmt.Errorf("hash member password: %w", err)
	}
	var output *DirectoryUser
	err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := validateIdentity(ctx, tx, identity); err != nil {
			return err
		}
		user := &servermodels.User{
			OrganizationID: identity.Organization.ID,
			Email:          input.Email, DisplayName: input.DisplayName, PasswordHash: passwordHash,
			Role: string(input.Role), Status: "active", Locale: identity.User.Locale, TimeZone: identity.User.TimeZone,
		}
		_, err := tx.NewInsert().Model(user).
			Column("organization_id", "email", "display_name", "password_hash", "role", "status", "locale", "time_zone").
			Returning("id").Exec(ctx)
		if isUniqueViolation(err) {
			return &ValidationError{Fields: map[string]ValidationCode{"email": ValidationEmailDuplicate}}
		}
		if err != nil {
			return err
		}
		if err := replaceUserTeams(ctx, tx, identity, user.ID, input.TeamIDs); err != nil {
			return err
		}
		output, err = loadDirectoryUser(ctx, tx, identity.Organization.ID, user.ID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return output, nil
}
