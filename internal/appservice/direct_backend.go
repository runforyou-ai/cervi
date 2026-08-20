//go:build server

package appservice

import (
	"context"
	"log/slog"

	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	contactaction "github.com/runforyou-ai/cervi/internal/actions/contact"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

var (
	_ Backend            = (*DirectBackend)(nil)
	_ WorkspaceInstaller = (*DirectBackend)(nil)
)

// DirectBackend 在服务端进程内直接调用 Action 和 Query。
type DirectBackend struct {
	installWorkspace                  *installationaction.InstallWorkspaceAction
	login                             *authaction.LoginAction
	logout                            *authaction.LogoutAction
	resolveIdentity                   *authaction.ResolveIdentityQuery
	installation                      *installationaction.StatusQuery
	loadInbox                         *inboxaction.LoadInboxQuery
	listWebsiteChannels               *channelaction.ListWebsiteChannelsQuery
	getWebsiteChannel                 *channelaction.GetWebsiteChannelQuery
	createWebsiteChannel              *channelaction.CreateWebsiteChannelAction
	updateWebsiteChannel              *channelaction.UpdateWebsiteChannelAction
	updateWebsiteChannelChatInterface *channelaction.UpdateWebsiteChannelChatInterfaceAction
	deleteWebsiteChannel              *channelaction.DeleteWebsiteChannelAction
	restoreWebsiteChannel             *channelaction.RestoreWebsiteChannelAction
	listChannels                      *channelaction.ListChannelsQuery
	listUsers                         *useraction.ListUsersQuery
	getUser                           *useraction.GetUserQuery
	updateProfile                     *useraction.UpdateProfileAction
	listContacts                      *contactaction.ListContactsQuery
	getContact                        *contactaction.GetContactQuery
	createContact                     *contactaction.CreateContactAction
	updateContact                     *contactaction.UpdateContactAction
	deleteContact                     *contactaction.DeleteContactAction
	restoreContact                    *contactaction.RestoreContactAction
	getS3Setting                      *settingaction.GetS3SettingQuery
	saveS3Setting                     *settingaction.SaveS3SettingAction
	testS3Setting                     *settingaction.TestS3SettingAction
}

// NewDirectBackend 创建直接访问服务端存储的应用后端。
func NewDirectBackend(db *bun.DB) *DirectBackend {
	return &DirectBackend{
		installWorkspace:                  installationaction.NewInstallWorkspaceAction(db),
		login:                             authaction.NewLoginAction(db),
		logout:                            authaction.NewLogoutAction(db),
		resolveIdentity:                   authaction.NewResolveIdentityQuery(db),
		installation:                      installationaction.NewStatusQuery(db),
		loadInbox:                         inboxaction.NewLoadInboxQuery(),
		listWebsiteChannels:               channelaction.NewListWebsiteChannelsQuery(db),
		getWebsiteChannel:                 channelaction.NewGetWebsiteChannelQuery(db),
		createWebsiteChannel:              channelaction.NewCreateWebsiteChannelAction(db),
		updateWebsiteChannel:              channelaction.NewUpdateWebsiteChannelAction(db),
		updateWebsiteChannelChatInterface: channelaction.NewUpdateWebsiteChannelChatInterfaceAction(db),
		deleteWebsiteChannel:              channelaction.NewDeleteWebsiteChannelAction(db),
		restoreWebsiteChannel:             channelaction.NewRestoreWebsiteChannelAction(db),
		listChannels:                      channelaction.NewListChannelsQuery(db),
		listUsers:                         useraction.NewListUsersQuery(db),
		getUser:                           useraction.NewGetUserQuery(db),
		updateProfile:                     useraction.NewUpdateProfileAction(db),
		listContacts:                      contactaction.NewListContactsQuery(db),
		getContact:                        contactaction.NewGetContactQuery(db),
		createContact:                     contactaction.NewCreateContactAction(db),
		updateContact:                     contactaction.NewUpdateContactAction(db),
		deleteContact:                     contactaction.NewDeleteContactAction(db),
		restoreContact:                    contactaction.NewRestoreContactAction(db),
		getS3Setting:                      settingaction.NewGetS3SettingQuery(db),
		saveS3Setting:                     settingaction.NewSaveS3SettingAction(db),
		testS3Setting:                     settingaction.NewTestS3SettingAction(),
	}
}

// requireInitialized 校验企业是否已完成初始化。
func (b *DirectBackend) requireInitialized(ctx context.Context, meta RequestMeta) error {
	status, err := b.InstallationStatus(ctx, meta)
	if err != nil {
		return err
	}
	if !status.Installed {
		return SessionError(meta, SessionStateSetup, cervii18n.ErrorInstallationRequired)
	}
	return nil
}

// authenticate 校验登录令牌并返回当前身份。
func (b *DirectBackend) authenticate(ctx context.Context, meta RequestMeta) (*servermodels.Identity, error) {
	if err := b.requireInitialized(ctx, meta); err != nil {
		return nil, err
	}
	if meta.Token == "" {
		return nil, SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	identity, err := b.resolveIdentity.Execute(ctx, meta.Token)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		slog.Warn("读取登录令牌失败", "error", err)
		return nil, FailedError(meta, cervii18n.ErrorAuthenticationStatusFailed)
	}
	if identity == nil {
		slog.Info("登录令牌无效")
		return nil, SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	return identity, nil
}
