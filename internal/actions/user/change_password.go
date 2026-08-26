//go:build server

package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	commonpassword "github.com/runforyou-ai/cervi/internal/common/password"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ChangePasswordAction 修改当前用户的登录密码。
type ChangePasswordAction struct {
	db *bun.DB
}

// NewChangePasswordAction 创建密码修改操作。
func NewChangePasswordAction(db *bun.DB) *ChangePasswordAction {
	return &ChangePasswordAction{db: db}
}

// Execute 在事务内核验当前密码并保存新密码。
func (a *ChangePasswordAction) Execute(ctx context.Context, identity *servermodels.Identity, input ChangePasswordInput) error {
	if fields := validateChangePasswordInput(input); len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		user := &servermodels.User{}
		err := tx.NewSelect().
			Model(user).
			Column("password_hash").
			Where("u.id = ?", identity.User.ID).
			Where("u.organization_id = ?", identity.Organization.ID).
			For("UPDATE").
			Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return common.ErrIdentityInvalid
		}
		if err != nil {
			return fmt.Errorf("read current password: %w", err)
		}
		if !commonpassword.Matches(user.PasswordHash, input.CurrentPassword) {
			return &ValidationError{Fields: map[string]ValidationCode{"currentPassword": ValidationCurrentPasswordIncorrect}}
		}
		passwordHash, err := commonpassword.Hash(input.NewPassword)
		if err != nil {
			return fmt.Errorf("hash new password: %w", err)
		}
		if _, err := tx.NewUpdate().
			Model(user).
			Set("password_hash = ?", passwordHash).
			Set("updated_at = now()").
			Where("u.id = ?", identity.User.ID).
			Where("u.organization_id = ?", identity.Organization.ID).
			Exec(ctx); err != nil {
			return fmt.Errorf("update password: %w", err)
		}
		return nil
	})
}
