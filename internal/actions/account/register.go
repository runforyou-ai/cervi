//go:build server

package account

import (
	"context"
	"errors"
	"fmt"
	"strings"

	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	deploymentaction "github.com/runforyou-ai/cervi/internal/actions/deployment"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	commonemail "github.com/runforyou-ai/cervi/internal/common/email"
	commonpassword "github.com/runforyou-ai/cervi/internal/common/password"
	"github.com/uptrace/bun"
)

// ErrRegistrationClosed 表示部署未开放账号注册。
var ErrRegistrationClosed = errors.New("account registration is closed")

// RegisterAction 注册本地账号并签发登录会话。
type RegisterAction struct {
	db *bun.DB
}

// NewRegisterAction 创建本地账号注册操作。
func NewRegisterAction(db *bun.DB) *RegisterAction {
	return &RegisterAction{db: db}
}

// Execute 在部署开放注册时校验字段、创建账号并签发登录会话。
func (a *RegisterAction) Execute(ctx context.Context, input NewAccountInput) (authaction.SessionOutput, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = commonemail.Normalize(input.Email)
	if fields := ValidateNewAccount(input); len(fields) > 0 {
		return authaction.SessionOutput{}, &ValidationError{Fields: fields}
	}
	passwordHash, err := commonpassword.Hash(input.Password)
	if err != nil {
		return authaction.SessionOutput{}, fmt.Errorf("hash account password: %w", err)
	}
	var output authaction.SessionOutput
	err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		settings, err := deploymentaction.LoadSettings(ctx, tx)
		if err != nil {
			return err
		}
		if !settings.RegistrationOpen {
			return ErrRegistrationClosed
		}
		created, err := identityaction.CreateAccount(ctx, tx, identityaction.NewAccount{
			Email: input.Email, PasswordHash: passwordHash, DisplayName: input.DisplayName, Locale: input.Locale, TimeZone: input.TimeZone,
		})
		if errors.Is(err, identityaction.ErrAccountEmailTaken) {
			return &ValidationError{Fields: map[string]ValidationCode{"email": ValidationEmailDuplicate}}
		}
		if err != nil {
			return err
		}
		output, err = authaction.IssueSession(ctx, tx, created)
		return err
	})
	if err != nil {
		return authaction.SessionOutput{}, err
	}
	return output, nil
}
