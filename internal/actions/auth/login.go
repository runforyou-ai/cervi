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

// Execute 校验账号密码并签发登录令牌。
func (a *LoginAction) Execute(ctx context.Context, input LoginInput) (LoginOutput, error) {
	user := &servermodels.User{}
	err := a.db.NewSelect().Model(user).
		ColumnExpr("u.id::text, u.organization_id::text, u.email, u.password_hash, u.role_id::text, u.locale, u.time_zone, u.work_status").
		ColumnExpr("om.display_name, om.status, om.avatar_file_id::text").
		Join("JOIN organization_members AS om ON om.id = u.id AND om.organization_id = u.organization_id AND om.type = 'user'").
		Where("lower(u.email) = lower(?)", commonemail.Normalize(input.Email)).
		Limit(1).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return LoginOutput{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginOutput{}, fmt.Errorf("find user: %w", err)
	}
	if user.Status != "active" || !commonpassword.Matches(user.PasswordHash, input.Password) {
		return LoginOutput{}, ErrInvalidCredentials
	}

	issued, identity, err := issueToken(ctx, a.db, user.ID)
	if err != nil {
		return LoginOutput{}, fmt.Errorf("issue login token: %w", err)
	}
	return LoginOutput{Identity: identity, Token: issued.Token, ExpiresAt: issued.ExpiresAt}, nil
}
