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
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/tenant"
	"github.com/uptrace/bun"
)

// authOps 持有企业初始化与登录会话的 Action 和 Query。
type authOps struct {
	installWorkspace *installationaction.InstallWorkspaceAction
	login            *authaction.LoginAction
	logout           *authaction.LogoutAction
}

// newAuthOps 创建企业初始化与登录会话的业务实现依赖。
func newAuthOps(db *bun.DB) authOps {
	return authOps{
		installWorkspace: installationaction.NewInstallWorkspaceAction(db),
		login:            authaction.NewLoginAction(db),
		logout:           authaction.NewLogoutAction(db),
	}
}

// InstallationStatus 返回服务端初始化状态和公开企业名称。
func (o *directOperations) InstallationStatus(ctx context.Context, meta RequestMeta) (InstallationStatus, error) {
	scope, err := o.resolveTenant.Resolve(ctx, tenant.AccessHost(ctx))
	if errors.Is(err, tenant.ErrNotFound) {
		return InstallationStatus{}, nil
	}
	if err != nil {
		if ctx.Err() != nil {
			return InstallationStatus{}, ctx.Err()
		}
		slog.Warn("读取初始化状态失败", "error", err)
		return InstallationStatus{}, FailedError(meta, cervii18n.ErrorInstallationStatusReadFailed)
	}
	return InstallationStatus{Installed: true, OrganizationName: scope.OrganizationName}, nil
}

// InstallWorkspace 创建企业管理员并返回登录令牌。
func (o *directOperations) InstallWorkspace(ctx context.Context, meta RequestMeta, input InstallWorkspaceInput) (Auth, error) {
	status, err := o.InstallationStatus(ctx, meta)
	if err != nil {
		return Auth{}, err
	}
	if status.Installed {
		slog.Info("企业已初始化")
		return Auth{}, SessionError(meta, SessionStateLogin, cervii18n.ErrorAlreadyInitialized).WithStatus(http.StatusConflict)
	}
	output, err := o.installWorkspace.Execute(ctx, installationaction.InstallWorkspaceInput{
		AccessHost:       tenant.AccessHost(ctx),
		OrganizationName: input.OrganizationName,
		DisplayName:      input.DisplayName,
		Email:            input.Email,
		Password:         input.Password,
		Locale:           domain.Locale(input.Locale),
		TimeZone:         input.TimeZone,
	})
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
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
	slog.Info("企业初始化完成", "organization_id", output.Identity.Organization.ID, "admin_id", output.Identity.User.ID)
	identity, err := o.identityFromModel(ctx, output.Identity)
	if err != nil {
		slog.Warn("读取初始化用户头像失败", "organization_id", output.Identity.Organization.ID, "error", err)
		return Auth{}, FailedError(meta, cervii18n.ErrorInstallationFailed)
	}
	return Auth{Identity: identity, Token: output.Token, ExpiresAt: output.ExpiresAt}, nil
}

// Login 校验账号密码并返回登录令牌。
func (o *directOperations) Login(ctx context.Context, meta RequestMeta, input LoginInput) (Auth, error) {
	scope, err := o.requireInitialized(ctx, meta)
	if err != nil {
		return Auth{}, err
	}
	output, err := o.login.Execute(ctx, authaction.LoginInput{OrganizationID: scope.OrganizationID, Email: input.Email, Password: input.Password})
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
	slog.Info("用户登录成功", "organization_id", output.Identity.Organization.ID, "user_id", output.Identity.User.ID, "work_status", domain.WorkStatusWorking)
	identity, err := o.identityFromModel(ctx, output.Identity)
	if err != nil {
		slog.Warn("读取登录用户头像失败", "organization_id", output.Identity.Organization.ID, "user_id", output.Identity.User.ID, "error", err)
		return Auth{}, FailedError(meta, cervii18n.ErrorLoginFailed)
	}
	return Auth{Identity: identity, Token: output.Token, ExpiresAt: output.ExpiresAt}, nil
}

// Logout 删除当前登录令牌。
func (o *directOperations) Logout(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) error {
	if err := o.logout.Execute(ctx, identity.Organization.ID, meta.Token); err != nil {
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
func (o *directOperations) LoadIdentity(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (Identity, error) {
	output, err := o.identityFromModel(ctx, identity)
	if err != nil {
		slog.Warn("读取当前用户头像失败", "organization_id", identity.Organization.ID, "user_id", identity.User.ID, "error", err)
		return Identity{}, FailedError(meta, cervii18n.ErrorUserReadFailed)
	}
	return output, nil
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
		installationaction.ValidationLocaleInvalid:            cervii18n.FieldLocaleInvalid,
		installationaction.ValidationTimeZoneInvalid:          cervii18n.FieldTimeZoneInvalid,
	}
	return translateValidationFields(fields, keys)
}
