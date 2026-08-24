//go:build server

package appservice

import (
	"context"
	"log/slog"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	aiprovideraction "github.com/runforyou-ai/cervi/internal/actions/aiprovider"
	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	contactaction "github.com/runforyou-ai/cervi/internal/actions/contact"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	memberaction "github.com/runforyou-ai/cervi/internal/actions/member"
	organizationaction "github.com/runforyou-ai/cervi/internal/actions/organization"
	roleaction "github.com/runforyou-ai/cervi/internal/actions/role"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	teamaction "github.com/runforyou-ai/cervi/internal/actions/team"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
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
	updateWebsiteChannelAccess        *channelaction.UpdateWebsiteChannelAccessAction
	updateWebsiteChannelStatus        *channelaction.UpdateWebsiteChannelStatusAction
	listChannels                      *channelaction.ListChannelsQuery
	listMemberOptions                 *memberaction.ListOptionsQuery
	createAgent                       *agentaction.CreateAgentAction
	listAgents                        *agentaction.ListAgentsQuery
	getAgent                          *agentaction.GetAgentQuery
	updateAgent                       *agentaction.UpdateAgentAction
	updateAgentStatus                 *agentaction.UpdateStatusAction
	updateAgentWorkStatus             *agentaction.UpdateWorkStatusAction
	listUsers                         *useraction.ListUsersQuery
	getUser                           *useraction.GetUserQuery
	createUser                        *useraction.CreateUserAction
	updateUser                        *useraction.UpdateUserAction
	updateUserRoles                   *useraction.UpdateRolesAction
	updateUserStatus                  *useraction.UpdateStatusAction
	listTeams                         *teamaction.ListTeamsQuery
	createTeam                        *teamaction.CreateTeamAction
	updateTeam                        *teamaction.UpdateTeamAction
	deleteTeam                        *teamaction.DeleteTeamAction
	listTeamMembers                   *teamaction.ListMembersQuery
	listTeamMemberCandidates          *teamaction.ListMemberCandidatesQuery
	addTeamMembers                    *teamaction.AddMembersAction
	removeTeamMembers                 *teamaction.RemoveMembersAction
	updateProfile                     *useraction.UpdateProfileAction
	changePassword                    *useraction.ChangePasswordAction
	updateUserPreferences             *useraction.UpdatePreferencesAction
	updateUserWorkStatus              *useraction.UpdateWorkStatusAction
	listContacts                      *contactaction.ListContactsQuery
	getContact                        *contactaction.GetContactQuery
	createContact                     *contactaction.CreateContactAction
	updateContact                     *contactaction.UpdateContactAction
	deleteContact                     *contactaction.DeleteContactAction
	restoreContact                    *contactaction.RestoreContactAction
	listRoles                         *roleaction.ListRolesQuery
	getRole                           *roleaction.GetRoleQuery
	createRole                        *roleaction.CreateRoleAction
	updateRole                        *roleaction.UpdateRoleAction
	deleteRole                        *roleaction.DeleteRoleAction
	listAIProviders                   *aiprovideraction.ListAIProvidersQuery
	getAIProvider                     *aiprovideraction.GetAIProviderQuery
	createAIProvider                  *aiprovideraction.CreateAIProviderAction
	updateAIProvider                  *aiprovideraction.UpdateAIProviderAction
	deleteAIProvider                  *aiprovideraction.DeleteAIProviderAction
	updateOrganization                *organizationaction.UpdateOrganizationAction
	getS3Setting                      *settingaction.GetS3SettingQuery
	saveS3Setting                     *settingaction.SaveS3SettingAction
	testS3Setting                     *settingaction.TestS3SettingAction
	createFileUpload                  *fileaction.CreateUploadAction
	getFile                           *fileaction.GetQuery
	markFileUploaded                  *fileaction.MarkUploadedAction
	localFiles                        *serverfilecontent.LocalStore
}

// NewDirectBackend 创建直接访问服务端存储的应用后端。
func NewDirectBackend(db *bun.DB, localFiles *serverfilecontent.LocalStore) *DirectBackend {
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
		updateWebsiteChannelAccess:        channelaction.NewUpdateWebsiteChannelAccessAction(db),
		updateWebsiteChannelStatus:        channelaction.NewUpdateWebsiteChannelStatusAction(db),
		listChannels:                      channelaction.NewListChannelsQuery(db),
		listMemberOptions:                 memberaction.NewListOptionsQuery(db),
		createAgent:                       agentaction.NewCreateAgentAction(db),
		listAgents:                        agentaction.NewListAgentsQuery(db),
		getAgent:                          agentaction.NewGetAgentQuery(db),
		updateAgent:                       agentaction.NewUpdateAgentAction(db),
		updateAgentStatus:                 agentaction.NewUpdateStatusAction(db),
		updateAgentWorkStatus:             agentaction.NewUpdateWorkStatusAction(db),
		listUsers:                         useraction.NewListUsersQuery(db),
		getUser:                           useraction.NewGetUserQuery(db),
		createUser:                        useraction.NewCreateUserAction(db),
		updateUser:                        useraction.NewUpdateUserAction(db),
		updateUserRoles:                   useraction.NewUpdateRolesAction(db),
		updateUserStatus:                  useraction.NewUpdateStatusAction(db),
		listTeams:                         teamaction.NewListTeamsQuery(db),
		createTeam:                        teamaction.NewCreateTeamAction(db),
		updateTeam:                        teamaction.NewUpdateTeamAction(db),
		deleteTeam:                        teamaction.NewDeleteTeamAction(db),
		listTeamMembers:                   teamaction.NewListMembersQuery(db),
		listTeamMemberCandidates:          teamaction.NewListMemberCandidatesQuery(db),
		addTeamMembers:                    teamaction.NewAddMembersAction(db),
		removeTeamMembers:                 teamaction.NewRemoveMembersAction(db),
		updateProfile:                     useraction.NewUpdateProfileAction(db),
		changePassword:                    useraction.NewChangePasswordAction(db),
		updateUserPreferences:             useraction.NewUpdatePreferencesAction(db),
		updateUserWorkStatus:              useraction.NewUpdateWorkStatusAction(db),
		listContacts:                      contactaction.NewListContactsQuery(db),
		getContact:                        contactaction.NewGetContactQuery(db),
		createContact:                     contactaction.NewCreateContactAction(db),
		updateContact:                     contactaction.NewUpdateContactAction(db),
		deleteContact:                     contactaction.NewDeleteContactAction(db),
		restoreContact:                    contactaction.NewRestoreContactAction(db),
		listRoles:                         roleaction.NewListRolesQuery(db),
		getRole:                           roleaction.NewGetRoleQuery(db),
		createRole:                        roleaction.NewCreateRoleAction(db),
		updateRole:                        roleaction.NewUpdateRoleAction(db),
		deleteRole:                        roleaction.NewDeleteRoleAction(db),
		listAIProviders:                   aiprovideraction.NewListAIProvidersQuery(db),
		getAIProvider:                     aiprovideraction.NewGetAIProviderQuery(db),
		createAIProvider:                  aiprovideraction.NewCreateAIProviderAction(db),
		updateAIProvider:                  aiprovideraction.NewUpdateAIProviderAction(db),
		deleteAIProvider:                  aiprovideraction.NewDeleteAIProviderAction(db),
		updateOrganization:                organizationaction.NewUpdateOrganizationAction(db),
		getS3Setting:                      settingaction.NewGetS3SettingQuery(db),
		saveS3Setting:                     settingaction.NewSaveS3SettingAction(db),
		testS3Setting:                     settingaction.NewTestS3SettingAction(),
		createFileUpload:                  fileaction.NewCreateUploadAction(db),
		getFile:                           fileaction.NewGetQuery(db),
		markFileUploaded:                  fileaction.NewMarkUploadedAction(db),
		localFiles:                        localFiles,
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
