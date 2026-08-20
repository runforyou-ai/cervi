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

// LoginOutput 返回登录身份和新会话。
type LoginOutput struct {
	Identity  *servermodels.Identity
	Token     string
	ExpiresAt time.Time
}

// NewLoginAction 创建用户登录操作。
func NewLoginAction(db *bun.DB) *LoginAction {
	return &LoginAction{db: db}
}

// Execute 校验账号密码并创建登录会话。
func (a *LoginAction) Execute(ctx context.Context, input LoginInput) (LoginOutput, error) {
	user := &servermodels.User{}
	err := a.db.NewSelect().
		Model(user).
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

	issued, identity, err := createSession(ctx, a.db, user.ID)
	if err != nil {
		return LoginOutput{}, fmt.Errorf("create login session: %w", err)
	}
	return LoginOutput{Identity: identity, Token: issued.Token, ExpiresAt: issued.ExpiresAt}, nil
}
