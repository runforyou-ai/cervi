//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	"github.com/runforyou-ai/cervi/internal/common"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// InstallationStatus 返回服务端初始化状态和公开企业名称。
func (b *DirectBackend) InstallationStatus(ctx context.Context, meta RequestMeta) (InstallationStatus, error) {
	status, err := b.installation.Execute(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return InstallationStatus{}, ctx.Err()
		}
		slog.Warn("读取初始化状态失败", "error", err)
		return InstallationStatus{}, FailedError(meta, cervii18n.ErrorInstallationStatusReadFailed)
	}
	return InstallationStatus{Installed: status.Installed, OrganizationName: status.OrganizationName}, nil
}

// InstallWorkspace 创建企业所有者并返回登录令牌。
func (b *DirectBackend) InstallWorkspace(ctx context.Context, meta RequestMeta, input InstallWorkspaceInput) (Auth, error) {
	status, err := b.InstallationStatus(ctx, meta)
	if err != nil {
		return Auth{}, err
	}
	if status.Installed {
		slog.Info("企业已初始化")
		return Auth{}, SessionError(meta, SessionStateLogin, cervii18n.ErrorAlreadyInitialized).WithStatus(http.StatusConflict)
	}
	output, err := b.installWorkspace.Execute(ctx, installationaction.InstallWorkspaceInput{
		OrganizationName: input.OrganizationName,
		DisplayName:      input.DisplayName,
		Email:            input.Email,
		Password:         input.Password,
	})
	var validationError *common.FieldError
	if errors.As(err, &validationError) {
		return Auth{}, InvalidError(meta, cervii18n.ErrorValidationFailed, installationFieldKeys(validationError.Fields))
	}
	if errors.Is(err, installationaction.ErrAlreadyInstalled) {
		slog.Info("企业已初始化")
		return Auth{}, SessionError(meta, SessionStateLogin, cervii18n.ErrorAlreadyInitialized).WithStatus(http.StatusConflict)
	}
	if err != nil {
		if ctx.Err() != nil {
			return Auth{}, ctx.Err()
		}
		slog.Warn("初始化企业失败", "error", err)
		return Auth{}, FailedError(meta, cervii18n.ErrorInstallationFailed)
	}
	slog.Info("企业初始化完成", "organization_id", output.Identity.Organization.ID, "owner_id", output.Identity.User.ID)
	return Auth{Identity: identityFromModel(output.Identity), Token: output.Token, ExpiresAt: output.ExpiresAt}, nil
}

// Login 校验账号密码并返回登录令牌。
func (b *DirectBackend) Login(ctx context.Context, meta RequestMeta, input LoginInput) (Auth, error) {
	if err := b.requireInitialized(ctx, meta); err != nil {
		return Auth{}, err
	}
	output, err := b.login.Execute(ctx, authaction.LoginInput{Email: input.Email, Password: input.Password})
	if errors.Is(err, authaction.ErrInvalidCredentials) {
		return Auth{}, InvalidError(meta, cervii18n.ErrorInvalidCredentials, nil)
	}
	if err != nil {
		if ctx.Err() != nil {
			return Auth{}, ctx.Err()
		}
		slog.Warn("用户登录失败", "error", err)
		return Auth{}, FailedError(meta, cervii18n.ErrorLoginFailed)
	}
	slog.Info("用户登录成功", "organization_id", output.Identity.Organization.ID, "user_id", output.Identity.User.ID)
	return Auth{Identity: identityFromModel(output.Identity), Token: output.Token, ExpiresAt: output.ExpiresAt}, nil
}

// Logout 删除当前登录令牌。
func (b *DirectBackend) Logout(ctx context.Context, meta RequestMeta) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.logout.Execute(ctx, meta.Token); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.Warn("删除登录令牌失败", "user_id", identity.User.ID, "error", err)
		return FailedError(meta, cervii18n.ErrorLogoutFailed)
	}
	slog.Info("用户退出登录", "organization_id", identity.Organization.ID, "user_id", identity.User.ID)
	return nil
}

// LoadIdentity 返回令牌对应的当前身份。
func (b *DirectBackend) LoadIdentity(ctx context.Context, meta RequestMeta) (Identity, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Identity{}, err
	}
	return identityFromModel(identity), nil
}

// installationFieldKeys 把初始化校验错误码映射为本地化文案键。
func installationFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		installationaction.ValidationOrganizationNameRequired: cervii18n.FieldOrganizationNameRequired,
		installationaction.ValidationOrganizationNameTooLong:  cervii18n.FieldOrganizationNameTooLong,
		installationaction.ValidationDisplayNameRequired:      cervii18n.FieldDisplayNameRequired,
		installationaction.ValidationEmailInvalid:             cervii18n.FieldEmailInvalid,
		installationaction.ValidationPasswordTooShort:         cervii18n.FieldPasswordTooShort,
		installationaction.ValidationPasswordTooLong:          cervii18n.FieldPasswordTooLong,
	}
	return translateValidationFields(fields, keys)
}
