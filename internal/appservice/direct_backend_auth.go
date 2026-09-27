//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	accountaction "github.com/runforyou-ai/cervi/internal/actions/account"
	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	invitationaction "github.com/runforyou-ai/cervi/internal/actions/invitation"
	organizationaction "github.com/runforyou-ai/cervi/internal/actions/organization"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/officialidentity"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// authOps 持有首次安装、账号会话和工作区列表的 Action 和 Query；官方账号登录只在配置官方身份服务时可用。
type authOps struct {
	registrationOpen      bool
	installWorkspace      *installationaction.InstallWorkspaceAction
	login                 *authaction.LoginAction
	register              *accountaction.RegisterAction
	logout                *authaction.LogoutAction
	changePassword        *accountaction.ChangePasswordAction
	startOfficialLogin    *authaction.StartOfficialLoginAction
	completeOfficialLogin *authaction.CompleteOfficialLoginAction
	listWorkspaces        *organizationaction.ListAccountWorkspacesQuery
	createWorkspace       *organizationaction.CreateWorkspaceAction
}

// newAuthOps 创建首次安装、账号会话和工作区入口的业务实现依赖，注册开关取自部署配置。
func newAuthOps(db *bun.DB, deployment DirectDeploymentConfig) authOps {
	ops := authOps{
		registrationOpen: deployment.RegistrationOpen,
		installWorkspace: installationaction.NewInstallWorkspaceAction(db),
		login:            authaction.NewLoginAction(db),
		register:         accountaction.NewRegisterAction(db, deployment.RegistrationOpen),
		logout:           authaction.NewLogoutAction(db),
		changePassword:   accountaction.NewChangePasswordAction(db),
		listWorkspaces:   organizationaction.NewListAccountWorkspacesQuery(db),
		createWorkspace:  organizationaction.NewCreateWorkspaceAction(db),
	}
	if deployment.OfficialIdentity != nil {
		redirectURI := deployment.PublicURL + authaction.OfficialLoginCallbackPath
		ops.startOfficialLogin = authaction.NewStartOfficialLoginAction(db, deployment.OfficialIdentity, redirectURI)
		ops.completeOfficialLogin = authaction.NewCompleteOfficialLoginAction(db, deployment.OfficialIdentity)
	}
	return ops
}

// authFromSession 把新签发的登录会话转换为应用契约。
func authFromSession(output authaction.SessionOutput) Auth {
	return Auth{Account: accountFromModel(*output.Account), Token: output.Token, ExpiresAt: output.ExpiresAt}
}

// InstallationStatus 返回部署的首次安装状态、注册开关和部署形态。
func (o *directOperations) InstallationStatus(ctx context.Context, meta RequestMeta) (InstallationStatus, error) {
	installed, err := o.installationStatus.Execute(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return InstallationStatus{}, ctx.Err()
		}
		slog.Warn("读取安装状态失败", "error", err)
		return InstallationStatus{}, FailedError(meta, cervii18n.ErrorInstallationStatusReadFailed)
	}
	return InstallationStatus{Installed: installed, RegistrationOpen: o.registrationOpen, DeploymentMode: DeploymentMode(o.deploymentMode)}, nil
}

// InstallWorkspace 在自托管部署尚无账号时创建部署管理员和第一个工作区，并返回登录会话。
func (o *directOperations) InstallWorkspace(ctx context.Context, meta RequestMeta, input InstallWorkspaceInput) (Auth, error) {
	if o.deploymentMode.Managed() {
		return Auth{}, InvalidError(meta, cervii18n.ErrorInstallationNotAvailable, nil)
	}
	output, err := o.installWorkspace.Execute(ctx, installationaction.InstallWorkspaceInput{
		WorkspaceName: input.WorkspaceName,
		WorkspaceSlug: input.WorkspaceSlug,
		DisplayName:   input.DisplayName,
		Email:         input.Email,
		Password:      input.Password,
		Locale:        domain.Locale(input.Locale),
		TimeZone:      input.TimeZone,
	})
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		return Auth{}, InvalidError(meta, cervii18n.ErrorValidationFailed, accountFieldKeys(validationError.Fields))
	}
	if errors.Is(err, installationaction.ErrAlreadyInstalled) {
		slog.Info("部署已完成首次安装")
		return Auth{}, SessionError(meta, SessionStateLogin, cervii18n.ErrorAlreadyInitialized).WithStatus(http.StatusConflict)
	}
	if err != nil {
		if ctx.Err() != nil {
			return Auth{}, ctx.Err()
		}
		slog.Warn("首次安装失败", "error", err)
		return Auth{}, FailedError(meta, cervii18n.ErrorInstallationFailed)
	}
	slog.Info("首次安装完成", "organization_id", output.Identity.Organization.ID, "account_id", output.Identity.Account.ID)
	return authFromSession(output.Session), nil
}

// Login 校验账号密码并返回登录会话。
func (o *directOperations) Login(ctx context.Context, meta RequestMeta, input LoginInput) (Auth, error) {
	if o.deploymentMode.Managed() {
		return Auth{}, InvalidError(meta, cervii18n.ErrorInvalidCredentials, nil)
	}
	output, err := o.login.Execute(ctx, authaction.LoginInput{Email: input.Email, Password: input.Password})
	if errors.Is(err, authaction.ErrInvalidCredentials) {
		return Auth{}, InvalidError(meta, cervii18n.ErrorInvalidCredentials, nil)
	}
	if err != nil {
		if ctx.Err() != nil {
			return Auth{}, ctx.Err()
		}
		slog.Warn("账号登录失败", "error", err)
		return Auth{}, FailedError(meta, cervii18n.ErrorLoginFailed)
	}
	slog.Info("账号登录成功", "account_id", output.Account.ID)
	return authFromSession(output), nil
}

// Register 在自托管部署开放注册时注册本地账号并返回登录会话。
func (o *directOperations) Register(ctx context.Context, meta RequestMeta, input RegisterInput) (Auth, error) {
	if o.deploymentMode.Managed() {
		return Auth{}, InvalidError(meta, cervii18n.ErrorRegistrationClosed, nil)
	}
	output, err := o.register.Execute(ctx, accountaction.NewAccountInput{
		DisplayName: input.DisplayName,
		Email:       input.Email,
		Password:    input.Password,
		Locale:      domain.Locale(input.Locale),
		TimeZone:    input.TimeZone,
	}, input.InvitationToken)
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		return Auth{}, InvalidError(meta, cervii18n.ErrorValidationFailed, accountFieldKeys(validationError.Fields))
	}
	if errors.Is(err, accountaction.ErrRegistrationClosed) {
		return Auth{}, InvalidError(meta, cervii18n.ErrorRegistrationClosed, nil)
	}
	if errors.Is(err, invitationaction.ErrInvitationInvalid) {
		return Auth{}, InvalidError(meta, cervii18n.ErrorInvitationInvalid, nil)
	}
	if errors.Is(err, invitationaction.ErrEmailMismatch) {
		return Auth{}, InvalidError(meta, cervii18n.ErrorInvitationEmailMismatch, nil)
	}
	if errors.Is(err, accountaction.ErrInstallationRequired) {
		return Auth{}, SessionError(meta, SessionStateSetup, cervii18n.ErrorInstallationRequired)
	}
	if err != nil {
		if ctx.Err() != nil {
			return Auth{}, ctx.Err()
		}
		slog.Warn("注册账号失败", "error", err)
		return Auth{}, FailedError(meta, cervii18n.ErrorRegistrationFailed)
	}
	slog.Info("账号注册成功", "account_id", output.Account.ID)
	return authFromSession(output), nil
}

// StartOfficialLogin 登记官方账号登录尝试并返回授权地址。
func (o *directOperations) StartOfficialLogin(ctx context.Context, meta RequestMeta, input OfficialLoginInput) (OfficialLoginStart, error) {
	if o.startOfficialLogin == nil {
		return OfficialLoginStart{}, InvalidError(meta, cervii18n.ErrorOfficialLoginNotAvailable, nil)
	}
	output, err := o.startOfficialLogin.Execute(ctx, authaction.StartOfficialLoginInput{
		State:         input.State,
		Nonce:         input.Nonce,
		CodeChallenge: input.CodeChallenge,
	})
	if err != nil {
		return OfficialLoginStart{}, officialLoginError(ctx, meta, "发起官方账号登录失败", err)
	}
	return OfficialLoginStart{AttemptID: output.AttemptID, AuthorizationURL: output.AuthorizationURL}, nil
}

// CompleteOfficialLogin 用授权码完成官方账号登录并返回登录会话。
func (o *directOperations) CompleteOfficialLogin(ctx context.Context, meta RequestMeta, input OfficialLoginCompletion) (Auth, error) {
	if o.completeOfficialLogin == nil {
		return Auth{}, InvalidError(meta, cervii18n.ErrorOfficialLoginNotAvailable, nil)
	}
	output, err := o.completeOfficialLogin.Execute(ctx, authaction.CompleteOfficialLoginInput{
		AttemptID:    input.AttemptID,
		Code:         input.Code,
		CodeVerifier: input.CodeVerifier,
		Locale:       domain.Locale(meta.Locale),
	})
	if err != nil {
		return Auth{}, officialLoginError(ctx, meta, "完成官方账号登录失败", err)
	}
	slog.Info("官方账号登录成功", "account_id", output.Account.ID)
	return authFromSession(output), nil
}

// officialLoginError 把官方账号登录的 Action 错误转成本地化业务错误。
func officialLoginError(ctx context.Context, meta RequestMeta, message string, err error) error {
	switch {
	case errors.Is(err, authaction.ErrOfficialLoginInputInvalid):
		return InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
	case errors.Is(err, authaction.ErrLoginAttemptInvalid):
		return InvalidError(meta, cervii18n.ErrorOfficialLoginExpired, nil)
	case errors.Is(err, authaction.ErrOfficialAccountUnavailable):
		return InvalidError(meta, cervii18n.ErrorOfficialAccountUnavailable, nil)
	case errors.Is(err, officialidentity.ErrRejected):
		slog.Warn(message, "error", err)
		return InvalidError(meta, cervii18n.ErrorOfficialLoginRejected, nil)
	case errors.Is(err, officialidentity.ErrUnavailable):
		slog.Warn(message, "error", err)
		return UnavailableError(meta, cervii18n.ErrorOfficialIdentityUnavailable, nil)
	case ctx.Err() != nil:
		return ctx.Err()
	default:
		slog.Warn(message, "error", err)
		return FailedError(meta, cervii18n.ErrorLoginFailed)
	}
}

// Logout 删除当前登录会话。
func (o *directOperations) Logout(ctx context.Context, meta RequestMeta, account *servermodels.AccountIdentity) error {
	if err := o.logout.Execute(ctx, account); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.Warn("删除登录会话失败", "account_id", account.Account.ID, "error", err)
		return FailedError(meta, cervii18n.ErrorLogoutFailed)
	}
	slog.Info("账号退出登录", "account_id", account.Account.ID)
	return nil
}

// LoadAccount 返回当前登录账号。
func (o *directOperations) LoadAccount(_ context.Context, _ RequestMeta, account *servermodels.AccountIdentity) (Account, error) {
	return accountFromModel(account.Account), nil
}

// ChangePassword 核验当前账号的密码并保存新密码，其他登录会话随之失效。
func (o *directOperations) ChangePassword(ctx context.Context, meta RequestMeta, account *servermodels.AccountIdentity, input ChangePasswordInput) error {
	err := o.changePassword.Execute(ctx, account, accountaction.ChangePasswordInput{
		CurrentPassword: input.CurrentPassword,
		NewPassword:     input.NewPassword,
	})
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, accountFieldKeys(validationError.Fields))
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.Warn("修改密码失败", "account_id", account.Account.ID, "error", err)
		return FailedError(meta, cervii18n.ErrorPasswordUpdateFailed)
	}
	slog.Info("密码修改成功", "account_id", account.Account.ID)
	return nil
}

// ListWorkspaces 返回当前账号作为有效成员可进入的工作区。
func (o *directOperations) ListWorkspaces(ctx context.Context, meta RequestMeta, account *servermodels.AccountIdentity) (WorkspaceList, error) {
	workspaces, err := o.listWorkspaces.Execute(ctx, account)
	if err != nil {
		if ctx.Err() != nil {
			return WorkspaceList{}, ctx.Err()
		}
		slog.Warn("读取工作区列表失败", "account_id", account.Account.ID, "error", err)
		return WorkspaceList{}, FailedError(meta, cervii18n.ErrorWorkspaceListFailed)
	}
	items := make([]Workspace, 0, len(workspaces))
	for _, workspace := range workspaces {
		items = append(items, Workspace{ID: workspace.ID, Name: workspace.Name, Slug: workspace.Slug})
	}
	return WorkspaceList{Items: items}, nil
}

// CreateWorkspace 创建工作区，当前账号成为首位管理员成员。
func (o *directOperations) CreateWorkspace(ctx context.Context, meta RequestMeta, account *servermodels.AccountIdentity, input WorkspaceInput) (Workspace, error) {
	workspace, err := o.createWorkspace.Execute(ctx, account, organizationaction.WorkspaceInput{Name: input.Name, Slug: input.Slug})
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		return Workspace{}, InvalidError(meta, cervii18n.ErrorValidationFailed, workspaceFieldKeys(validationError.Fields))
	}
	if err != nil {
		if ctx.Err() != nil {
			return Workspace{}, ctx.Err()
		}
		slog.Warn("创建工作区失败", "account_id", account.Account.ID, "error", err)
		return Workspace{}, FailedError(meta, cervii18n.ErrorWorkspaceCreateFailed)
	}
	slog.Info("工作区已创建", "organization_id", workspace.ID, "account_id", account.Account.ID)
	return Workspace{ID: workspace.ID, Name: workspace.Name, Slug: workspace.Slug}, nil
}

// LoadIdentity 返回当前账号在请求目标工作区中的成员身份。
func (o *directOperations) LoadIdentity(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (Identity, error) {
	output, err := o.identityFromModel(ctx, identity)
	if err != nil {
		slog.Warn("读取当前成员头像失败", "organization_id", identity.Organization.ID, "user_id", identity.User.ID, "error", err)
		return Identity{}, FailedError(meta, cervii18n.ErrorUserReadFailed)
	}
	return output, nil
}

// accountFieldKeys 把账号与首次安装的校验错误码映射为本地化文案键。
func accountFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		accountaction.ValidationDisplayNameRequired:      cervii18n.FieldDisplayNameRequired,
		accountaction.ValidationDisplayNameInvalid:       cervii18n.FieldDisplayNameInvalid,
		accountaction.ValidationEmailInvalid:             cervii18n.FieldEmailInvalid,
		accountaction.ValidationEmailDuplicate:           cervii18n.FieldEmailDuplicate,
		accountaction.ValidationPasswordTooShort:         cervii18n.FieldPasswordTooShort,
		accountaction.ValidationPasswordTooLong:          cervii18n.FieldPasswordTooLong,
		accountaction.ValidationCurrentPasswordIncorrect: cervii18n.FieldCurrentPasswordIncorrect,
		accountaction.ValidationLocaleInvalid:            cervii18n.FieldLocaleInvalid,
		accountaction.ValidationTimeZoneInvalid:          cervii18n.FieldTimeZoneInvalid,
		organizationaction.ValidationNameRequired:        cervii18n.FieldOrganizationNameRequired,
		organizationaction.ValidationNameTooLong:         cervii18n.FieldOrganizationNameTooLong,
		organizationaction.ValidationSlugInvalid:         cervii18n.FieldWorkspaceSlugInvalid,
		organizationaction.ValidationSlugTaken:           cervii18n.FieldWorkspaceSlugTaken,
	}
	return translateValidationFields(fields, keys)
}

// workspaceFieldKeys 把工作区名称和标识的校验错误码映射为本地化文案键。
func workspaceFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		organizationaction.ValidationNameRequired: cervii18n.FieldOrganizationNameRequired,
		organizationaction.ValidationNameTooLong:  cervii18n.FieldOrganizationNameTooLong,
		organizationaction.ValidationSlugInvalid:  cervii18n.FieldWorkspaceSlugInvalid,
		organizationaction.ValidationSlugTaken:    cervii18n.FieldWorkspaceSlugTaken,
	}
	return translateValidationFields(fields, keys)
}
