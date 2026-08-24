//go:build server

package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	commonemail "github.com/runforyou-ai/cervi/internal/common/email"
	commonpassword "github.com/runforyou-ai/cervi/internal/common/password"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

// LoginAction 执行用户登录操作。
type LoginAction struct {
	db *bun.DB
}

// LoginInput 定义登录操作输入。
type LoginInput struct {
	Email    string
	Password string
}

// LoginOutput 返回登录身份和新令牌。
type LoginOutput struct {
	Identity  *servermodels.Identity
	Token     string
	ExpiresAt time.Time
}

// NewLoginAction 创建用户登录操作。
func NewLoginAction(db *bun.DB) *LoginAction {
	return &LoginAction{db: db}
}

// Execute 校验账号密码，将工作状态切换为工作中并签发登录令牌。
func (a *LoginAction) Execute(ctx context.Context, input LoginInput) (LoginOutput, error) {
	user := &servermodels.User{}
	err := a.db.NewSelect().Model(user).
		ColumnExpr("u.id::text, u.identity_id::text, u.organization_id::text, u.email, u.password_hash, u.role_id::text, u.status, u.locale, u.time_zone").
		Join("JOIN organization_identities AS oi ON oi.id = u.identity_id AND oi.organization_id = u.organization_id AND oi.type = 'user'").
		Where("lower(u.email) = lower(?)", commonemail.Normalize(input.Email)).
		Limit(1).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return LoginOutput{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginOutput{}, fmt.Errorf("find user: %w", err)
	}
	if user.Status != string(domain.UserStatusActive) || !commonpassword.Matches(user.PasswordHash, input.Password) {
		return LoginOutput{}, ErrInvalidCredentials
	}

	var output LoginOutput
	err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewUpdate().Model((*servermodels.OrganizationIdentity)(nil)).
			Set("work_status = ?", domain.WorkStatusWorking).
			Set("work_status_updated_at = now()").
			Set("updated_at = now()").
			Where("organization_id = ?", user.OrganizationID).
			Where("id = ?", user.IdentityID).
			Where("type = ?", domain.OrganizationIdentityTypeUser).
			Exec(ctx); err != nil {
			return err
		}
		issued, identity, err := issueToken(ctx, tx, user.ID)
		if err != nil {
			return err
		}
		output = LoginOutput{Identity: identity, Token: issued.Token, ExpiresAt: issued.ExpiresAt}
		return nil
	})
	if err != nil {
		return LoginOutput{}, fmt.Errorf("complete login: %w", err)
	}
	return output, nil
}
