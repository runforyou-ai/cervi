//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"time"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	aiprovideraction "github.com/runforyou-ai/cervi/internal/actions/aiprovider"
	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	businesssystemaction "github.com/runforyou-ai/cervi/internal/actions/businesssystem"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	contactaction "github.com/runforyou-ai/cervi/internal/actions/contact"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	integrationconnectionaction "github.com/runforyou-ai/cervi/internal/actions/integrationconnection"
	knowledgebaseaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	memberaction "github.com/runforyou-ai/cervi/internal/actions/member"
	organizationaction "github.com/runforyou-ai/cervi/internal/actions/organization"
	roleaction "github.com/runforyou-ai/cervi/internal/actions/role"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	teamaction "github.com/runforyou-ai/cervi/internal/actions/team"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
	"github.com/runforyou-ai/cervi/internal/integration/connector"
	"github.com/runforyou-ai/cervi/internal/integration/modelprovider"
	"github.com/runforyou-ai/cervi/internal/integration/telegram"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/tenant"
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
	resolveTenant                     tenant.Resolver
	loadInbox                         *inboxaction.LoadInboxQuery
	listCustomerServiceAssignees      *inboxaction.ListCustomerServiceAssigneesQuery
	listConversationMessages          *conversationaction.ListConversationMessagesQuery
	markConversationRead              *conversationaction.MarkConversationReadAction
	sendCustomerTextMessage           *conversationaction.SendCustomerTextMessageAction
	claimServiceSession               *conversationaction.ClaimServiceSessionAction
	transferServiceSession            *conversationaction.TransferServiceSessionAction
	closeServiceSession               *conversationaction.CloseServiceSessionAction
	reopenServiceSession              *conversationaction.ReopenServiceSessionAction
	sendFirstDirectTextMessage        *conversationaction.SendFirstDirectTextMessageAction
	sendDirectTextMessage             *conversationaction.SendDirectTextMessageAction
	createGroupConversation           *conversationaction.CreateGroupConversationAction
	getGroupConversation              *conversationaction.GetGroupConversationQuery
	updateGroupConversation           *conversationaction.UpdateGroupConversationAction
	addGroupConversationMembers       *conversationaction.AddGroupConversationMembersAction
	removeGroupConversationMember     *conversationaction.RemoveGroupConversationMemberAction
	transferGroupConversationOwner    *conversationaction.TransferGroupConversationOwnerAction
	leaveGroupConversation            *conversationaction.LeaveGroupConversationAction
	sendGroupTextMessage              *conversationaction.SendGroupTextMessageAction
	listMessageChannels               *channelaction.ListMessageChannelsQuery
	getWebsiteChannel                 *channelaction.GetWebsiteChannelQuery
	getTelegramChannel                *channelaction.GetTelegramChannelQuery
	getMessageChannel                 *channelaction.GetMessageChannelQuery
	createMessageChannel              *channelaction.CreateMessageChannelAction
	updateMessageChannel              *channelaction.UpdateMessageChannelAction
	updateWebsiteChannelChatInterface *channelaction.UpdateWebsiteChannelChatInterfaceAction
	updateWebsiteChannelAccess        *channelaction.UpdateWebsiteChannelAccessAction
	testTelegramConnection            *channelaction.TestTelegramConnectionAction
	saveTelegramConnection            *channelaction.SaveTelegramConnectionAction
	updateTelegramChannelStatus       *channelaction.UpdateTelegramChannelStatusAction
	updateMessageChannelStatus        *channelaction.UpdateMessageChannelStatusAction
	listChannelOptions                *channelaction.ListChannelOptionsQuery
	listMemberOptions                 *memberaction.ListOptionsQuery
	listAgentModelOptions             *agentaction.ListModelOptionsQuery
	createAgent                       *agentaction.CreateAgentAction
	listAgents                        *agentaction.ListAgentsQuery
	getAgent                          *agentaction.GetAgentQuery
	updateAgent                       *agentaction.UpdateAgentAction
	updateAgentExecution              *agentaction.UpdateExecutionAction
	updateAgentStatus                 *agentaction.UpdateStatusAction
	updateAgentWorkStatus             *agentaction.UpdateWorkStatusAction
	listUsers                         *useraction.ListUsersQuery
	getUser                           *useraction.GetUserQuery
	createUser                        *useraction.CreateUserAction
	updateUser                        *useraction.UpdateUserAction
	updateRoleAssignments             *roleaction.UpdateAssignmentsAction
	updateUserStatus                  *useraction.UpdateStatusAction
	listTeams                         *teamaction.ListTeamsQuery
	createTeam                        *teamaction.CreateTeamAction
	updateTeam                        *teamaction.UpdateTeamAction
	deleteTeam                        *teamaction.DeleteTeamAction
	listKnowledgeBases                *knowledgebaseaction.ListKnowledgeBasesQuery
	listExternalKnowledgeBaseOptions  *knowledgebaseaction.ListExternalOptionsQuery
	listKnowledgeDocuments            *knowledgebaseaction.ListKnowledgeDocumentsQuery
	getKnowledgeDocument              *knowledgebaseaction.GetKnowledgeDocumentQuery
	listKnowledgeDocumentSegments     *knowledgebaseaction.ListKnowledgeDocumentSegmentsQuery
	retrieveKnowledgeBase             *knowledgebaseaction.RetrieveKnowledgeBaseQuery
	getKnowledgeBase                  *knowledgebaseaction.GetKnowledgeBaseQuery
	createKnowledgeBase               *knowledgebaseaction.CreateKnowledgeBaseAction
	updateKnowledgeBase               *knowledgebaseaction.UpdateKnowledgeBaseAction
	deleteKnowledgeBase               *knowledgebaseaction.DeleteKnowledgeBaseAction
	createKnowledgeGroup              *knowledgebaseaction.CreateKnowledgeGroupAction
	updateKnowledgeGroup              *knowledgebaseaction.UpdateKnowledgeGroupAction
	deleteKnowledgeGroup              *knowledgebaseaction.DeleteKnowledgeGroupAction
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
	testAIProviderConnection          *aiprovideraction.TestConnectionAction
	createAIProvider                  *aiprovideraction.CreateAIProviderAction
	updateAIProvider                  *aiprovideraction.UpdateAIProviderAction
	deleteAIProvider                  *aiprovideraction.DeleteAIProviderAction
	listBusinessSystems               *businesssystemaction.ListBusinessSystemsQuery
	getBusinessSystem                 *businesssystemaction.GetBusinessSystemQuery
	createBusinessSystem              *businesssystemaction.CreateBusinessSystemAction
	updateBusinessSystem              *businesssystemaction.UpdateBusinessSystemAction
	deleteBusinessSystem              *businesssystemaction.DeleteBusinessSystemAction
	listIntegrationConnections        *integrationconnectionaction.ListIntegrationConnectionsQuery
	getIntegrationConnection          *integrationconnectionaction.GetIntegrationConnectionQuery
	testIntegrationConnection         *integrationconnectionaction.TestConnectionAction
	createIntegrationConnection       *integrationconnectionaction.CreateIntegrationConnectionAction
	updateIntegrationConnection       *integrationconnectionaction.UpdateIntegrationConnectionAction
	deleteIntegrationConnection       *integrationconnectionaction.DeleteIntegrationConnectionAction
	updateOrganization                *organizationaction.UpdateOrganizationAction
	getS3Setting                      *settingaction.GetS3SettingQuery
	saveS3Setting                     *settingaction.SaveS3SettingAction
	testS3Setting                     *settingaction.TestS3SettingAction
	createFileUpload                  *fileaction.CreateUploadAction
	completeFileUpload                *fileaction.CompleteUploadAction
	getFile                           *fileaction.GetQuery
	localFiles                        *serverfilecontent.LocalStore
}

// NewDirectBackend 创建直接访问服务端存储的应用后端。
func NewDirectBackend(db *bun.DB, localFiles *serverfilecontent.LocalStore, tenantResolver tenant.Resolver, agentScheduler conversationaction.AgentMessageScheduler, agentCoordinator conversationaction.ServiceSessionAgentRunCoordinator) *DirectBackend {
	connectionRunner := connectiontest.NewRunner(10 * time.Second)
	connectionClient := connectiontest.NewHTTPClient()
	modelProviderRegistry := modelprovider.NewRegistry(connectionClient)
	connectorClient := connectionClient
	connectorRegistry := connector.NewRegistry(connectorClient)
	difyKnowledgeDocuments := connector.NewDifyKnowledgeDocumentLister(connectorClient)
	difyKnowledgeRetriever := connector.NewDifyKnowledgeRetriever(connectorClient)
	telegramAPI := telegram.NewClient(connectionClient)
	return &DirectBackend{
		installWorkspace:                  installationaction.NewInstallWorkspaceAction(db),
		login:                             authaction.NewLoginAction(db),
		logout:                            authaction.NewLogoutAction(db),
		resolveIdentity:                   authaction.NewResolveIdentityQuery(db),
		resolveTenant:                     tenantResolver,
		loadInbox:                         inboxaction.NewLoadInboxQuery(db),
		listCustomerServiceAssignees:      inboxaction.NewListCustomerServiceAssigneesQuery(db),
		listConversationMessages:          conversationaction.NewListConversationMessagesQuery(db),
		markConversationRead:              conversationaction.NewMarkConversationReadAction(db),
		sendCustomerTextMessage:           conversationaction.NewSendCustomerTextMessageAction(db),
		claimServiceSession:               conversationaction.NewClaimServiceSessionAction(db, agentCoordinator),
		transferServiceSession:            conversationaction.NewTransferServiceSessionAction(db, agentCoordinator, agentScheduler),
		closeServiceSession:               conversationaction.NewCloseServiceSessionAction(db, agentCoordinator),
		reopenServiceSession:              conversationaction.NewReopenServiceSessionAction(db),
		sendFirstDirectTextMessage:        conversationaction.NewSendFirstDirectTextMessageAction(db, agentScheduler),
		sendDirectTextMessage:             conversationaction.NewSendDirectTextMessageAction(db, agentScheduler),
		createGroupConversation:           conversationaction.NewCreateGroupConversationAction(db),
		getGroupConversation:              conversationaction.NewGetGroupConversationQuery(db),
		updateGroupConversation:           conversationaction.NewUpdateGroupConversationAction(db),
		addGroupConversationMembers:       conversationaction.NewAddGroupConversationMembersAction(db),
		removeGroupConversationMember:     conversationaction.NewRemoveGroupConversationMemberAction(db),
		transferGroupConversationOwner:    conversationaction.NewTransferGroupConversationOwnerAction(db),
		leaveGroupConversation:            conversationaction.NewLeaveGroupConversationAction(db),
		sendGroupTextMessage:              conversationaction.NewSendGroupTextMessageAction(db),
		listMessageChannels:               channelaction.NewListMessageChannelsQuery(db),
		getWebsiteChannel:                 channelaction.NewGetWebsiteChannelQuery(db),
		getTelegramChannel:                channelaction.NewGetTelegramChannelQuery(db),
		getMessageChannel:                 channelaction.NewGetMessageChannelQuery(db),
		createMessageChannel:              channelaction.NewCreateMessageChannelAction(db),
		updateMessageChannel:              channelaction.NewUpdateMessageChannelAction(db),
		updateWebsiteChannelChatInterface: channelaction.NewUpdateWebsiteChannelChatInterfaceAction(db),
		updateWebsiteChannelAccess:        channelaction.NewUpdateWebsiteChannelAccessAction(db),
		testTelegramConnection:            channelaction.NewTestTelegramConnectionAction(db, connectionRunner, telegramAPI),
		saveTelegramConnection:            channelaction.NewSaveTelegramConnectionAction(db, connectionRunner, telegramAPI),
		updateTelegramChannelStatus:       channelaction.NewUpdateTelegramChannelStatusAction(db, connectionRunner, telegramAPI),
		updateMessageChannelStatus:        channelaction.NewUpdateMessageChannelStatusAction(db),
		listChannelOptions:                channelaction.NewListChannelOptionsQuery(db),
		listMemberOptions:                 memberaction.NewListOptionsQuery(db),
		listAgentModelOptions:             agentaction.NewListModelOptionsQuery(db),
		createAgent:                       agentaction.NewCreateAgentAction(db),
		listAgents:                        agentaction.NewListAgentsQuery(db),
		getAgent:                          agentaction.NewGetAgentQuery(db),
		updateAgent:                       agentaction.NewUpdateAgentAction(db),
		updateAgentExecution:              agentaction.NewUpdateExecutionAction(db),
		updateAgentStatus:                 agentaction.NewUpdateStatusAction(db),
		updateAgentWorkStatus:             agentaction.NewUpdateWorkStatusAction(db),
		listUsers:                         useraction.NewListUsersQuery(db),
		getUser:                           useraction.NewGetUserQuery(db),
		createUser:                        useraction.NewCreateUserAction(db),
		updateUser:                        useraction.NewUpdateUserAction(db),
		updateRoleAssignments:             roleaction.NewUpdateAssignmentsAction(db),
		updateUserStatus:                  useraction.NewUpdateStatusAction(db),
		listTeams:                         teamaction.NewListTeamsQuery(db),
		createTeam:                        teamaction.NewCreateTeamAction(db),
		updateTeam:                        teamaction.NewUpdateTeamAction(db),
		deleteTeam:                        teamaction.NewDeleteTeamAction(db),
		listKnowledgeBases:                knowledgebaseaction.NewListKnowledgeBasesQuery(db),
		listExternalKnowledgeBaseOptions:  knowledgebaseaction.NewListExternalOptionsQuery(db, connector.NewDifyKnowledgeBaseLister(connectorClient)),
		listKnowledgeDocuments:            knowledgebaseaction.NewListKnowledgeDocumentsQuery(db, difyKnowledgeDocuments),
		getKnowledgeDocument:              knowledgebaseaction.NewGetKnowledgeDocumentQuery(db, difyKnowledgeDocuments),
		listKnowledgeDocumentSegments:     knowledgebaseaction.NewListKnowledgeDocumentSegmentsQuery(db, difyKnowledgeDocuments),
		retrieveKnowledgeBase:             knowledgebaseaction.NewRetrieveKnowledgeBaseQuery(db, difyKnowledgeRetriever),
		getKnowledgeBase:                  knowledgebaseaction.NewGetKnowledgeBaseQuery(db),
		createKnowledgeBase:               knowledgebaseaction.NewCreateKnowledgeBaseAction(db),
		updateKnowledgeBase:               knowledgebaseaction.NewUpdateKnowledgeBaseAction(db),
		deleteKnowledgeBase:               knowledgebaseaction.NewDeleteKnowledgeBaseAction(db),
		createKnowledgeGroup:              knowledgebaseaction.NewCreateKnowledgeGroupAction(db),
		updateKnowledgeGroup:              knowledgebaseaction.NewUpdateKnowledgeGroupAction(db),
		deleteKnowledgeGroup:              knowledgebaseaction.NewDeleteKnowledgeGroupAction(db),
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
		testAIProviderConnection:          aiprovideraction.NewTestConnectionAction(connectionRunner, modelProviderRegistry),
		createAIProvider:                  aiprovideraction.NewCreateAIProviderAction(db),
		updateAIProvider:                  aiprovideraction.NewUpdateAIProviderAction(db),
		deleteAIProvider:                  aiprovideraction.NewDeleteAIProviderAction(db),
		listBusinessSystems:               businesssystemaction.NewListBusinessSystemsQuery(db),
		getBusinessSystem:                 businesssystemaction.NewGetBusinessSystemQuery(db),
		createBusinessSystem:              businesssystemaction.NewCreateBusinessSystemAction(db),
		updateBusinessSystem:              businesssystemaction.NewUpdateBusinessSystemAction(db),
		deleteBusinessSystem:              businesssystemaction.NewDeleteBusinessSystemAction(db),
		listIntegrationConnections:        integrationconnectionaction.NewListIntegrationConnectionsQuery(db),
		getIntegrationConnection:          integrationconnectionaction.NewGetIntegrationConnectionQuery(db),
		testIntegrationConnection:         integrationconnectionaction.NewTestConnectionAction(db, connectionRunner, connectorRegistry),
		createIntegrationConnection:       integrationconnectionaction.NewCreateIntegrationConnectionAction(db),
		updateIntegrationConnection:       integrationconnectionaction.NewUpdateIntegrationConnectionAction(db),
		deleteIntegrationConnection:       integrationconnectionaction.NewDeleteIntegrationConnectionAction(db),
		updateOrganization:                organizationaction.NewUpdateOrganizationAction(db),
		getS3Setting:                      settingaction.NewGetS3SettingQuery(db),
		saveS3Setting:                     settingaction.NewSaveS3SettingAction(db),
		testS3Setting:                     settingaction.NewTestS3SettingAction(connectionRunner),
		createFileUpload:                  fileaction.NewCreateUploadAction(db),
		completeFileUpload:                fileaction.NewCompleteUploadAction(db),
		getFile:                           fileaction.NewGetQuery(db),
		localFiles:                        localFiles,
	}
}

// requireInitialized 解析当前请求的企业范围，并校验该企业是否已完成初始化。
func (b *DirectBackend) requireInitialized(ctx context.Context, meta RequestMeta) (tenant.Scope, error) {
	scope, err := b.resolveTenant.Resolve(ctx, tenant.AccessHost(ctx))
	if errors.Is(err, tenant.ErrNotFound) {
		return tenant.Scope{}, SessionError(meta, SessionStateSetup, cervii18n.ErrorInstallationRequired)
	}
	if err != nil {
		if ctx.Err() != nil {
			return tenant.Scope{}, ctx.Err()
		}
		slog.Warn("解析当前企业失败", "error", err)
		return tenant.Scope{}, FailedError(meta, cervii18n.ErrorInstallationStatusReadFailed)
	}
	return scope, nil
}

// authenticate 校验登录令牌并返回当前身份。
func (b *DirectBackend) authenticate(ctx context.Context, meta RequestMeta) (*servermodels.Identity, error) {
	scope, err := b.requireInitialized(ctx, meta)
	if err != nil {
		return nil, err
	}
	if meta.Token == "" {
		return nil, SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	identity, err := b.resolveIdentity.Execute(ctx, scope.OrganizationID, meta.Token)
	if errors.Is(err, authaction.ErrIdentityNotFound) {
		slog.Info("登录令牌无效")
		return nil, SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		slog.Warn("读取登录令牌失败", "error", err)
		return nil, FailedError(meta, cervii18n.ErrorAuthenticationStatusFailed)
	}
	return identity, nil
}
