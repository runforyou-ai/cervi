//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"time"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	invitationaction "github.com/runforyou-ai/cervi/internal/actions/invitation"
	knowledgebaseaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	mcpserveraction "github.com/runforyou-ai/cervi/internal/actions/mcpserver"
	translationaction "github.com/runforyou-ai/cervi/internal/actions/translation"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
	mcpintegration "github.com/runforyou-ai/cervi/internal/integration/mcp"
	"github.com/runforyou-ai/cervi/internal/integration/modelprovider"
	"github.com/runforyou-ai/cervi/internal/integration/telegram"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

var (
	_ Backend            = (*DirectBackend)(nil)
	_ WorkspaceInstaller = (*DirectBackend)(nil)
)

// sessionGuard 校验登录会话并解析请求目标工作区中的成员身份。
type sessionGuard struct {
	db                 *bun.DB
	deploymentMode     domain.DeploymentMode
	installationStatus *installationaction.StatusQuery
	resolveAccount     *authaction.ResolveAccountQuery
}

// DirectBackend 解析登录身份并把业务调用分发给已认证实现。
//
// 各 Backend 方法的认证分发由 appservicegen 生成到 direct_backend_gen.go；
// auth=public 的方法不解析身份，其余方法在调用实现前必须拿到当前身份。
type DirectBackend struct {
	ops *directOperations
}

// directOperations 持有已认证业务实现所需的 Action 和 Query。
type directOperations struct {
	sessionGuard
	authOps
	conversationOps
	inboxOps
	channelOps
	contactOps
	directoryOps
	customerServiceOps
	aiPerformanceOps
	knowledgeGapOps
	agentOps
	assistantOps
	knowledgeOps
	integrationOps
	deviceOps
	fileOps
	translationOps
	webSearchOps
	invitationOps
}

// DirectDeploymentConfig 定义直接后端的部署名称与形态、部署地址、注册开关、邀请邮件发送和官方身份服务；官方身份服务只在托管部署设置，邮件发送只在配置了 SMTP 时设置。
type DirectDeploymentConfig struct {
	Name             string
	Mode             domain.DeploymentMode
	PublicURL        string
	RegistrationOpen bool
	InvitationMailer invitationaction.Mailer
	OfficialIdentity authaction.OfficialIdentityProvider
}

// NewDirectBackend 创建直接访问服务端存储的应用后端。
func NewDirectBackend(db *bun.DB, deployment DirectDeploymentConfig, localFiles *serverfilecontent.LocalStore, s3 serverfilecontent.S3Config, agentScheduler conversationaction.AgentMessageScheduler, agentCoordinator *agentrunaction.ExecuteAction, taskEnqueuer servertask.TxEnqueuer, serviceReplySuggestions *agentrunaction.GenerateServiceReplySuggestionsAction, translator *translationaction.Translator) *DirectBackend {
	connectionRunner := connectiontest.NewRunner(10 * time.Second)
	connectionClient := connectiontest.NewHTTPClient()
	modelProviderRegistry := modelprovider.NewRegistry(connectionClient)
	telegramAPI := telegram.NewClient(connectionClient)
	mcpTest := mcpserveraction.NewTestConnectionAction(mcpintegration.NewClient())
	mcpScheduler := mcpserveraction.NewToolsScheduler(taskEnqueuer)
	guard := sessionGuard{db: db, deploymentMode: deployment.Mode, installationStatus: installationaction.NewStatusQuery(db), resolveAccount: authaction.NewResolveAccountQuery(db)}
	documentQuery := knowledgebaseaction.NewDocumentQuery(db)
	ops := &directOperations{
		sessionGuard:       guard,
		authOps:            newAuthOps(db, deployment),
		conversationOps:    newConversationOps(db, agentScheduler, agentCoordinator, taskEnqueuer),
		inboxOps:           newInboxOps(db, taskEnqueuer),
		channelOps:         newChannelOps(db, connectionRunner, telegramAPI),
		contactOps:         newContactOps(db),
		directoryOps:       newDirectoryOps(db, agentCoordinator, taskEnqueuer),
		customerServiceOps: newCustomerServiceOps(db),
		aiPerformanceOps:   newAIPerformanceOps(db),
		knowledgeGapOps:    newKnowledgeGapOps(db, taskEnqueuer),
		agentOps:           newAgentOps(db, agentCoordinator, serviceReplySuggestions),
		assistantOps:       newAssistantOps(db),
		knowledgeOps:       newKnowledgeOps(db, taskEnqueuer, documentQuery),
		integrationOps:     newIntegrationOps(db, connectionRunner, modelProviderRegistry, mcpTest, mcpScheduler),
		deviceOps:          newDeviceOps(db),
		fileOps:            newFileOps(db, localFiles, s3, serverfilecontent.NewLinks("", s3.PublicBaseURL)),
		translationOps:     newTranslationOps(db, translator),
		webSearchOps:       newWebSearchOps(db, connectionRunner),
		invitationOps:      newInvitationOps(db, deployment.InvitationMailer, deployment.PublicURL),
	}
	return &DirectBackend{ops: ops}
}

// InstallWorkspace 完成首次安装并返回部署管理员的登录会话。
func (b *DirectBackend) InstallWorkspace(ctx context.Context, meta RequestMeta, input InstallWorkspaceInput) (Auth, error) {
	return b.ops.InstallWorkspace(ctx, meta, input)
}

// AuthenticateMember 校验实时事件流请求携带的登录令牌并返回成员会话。
func (b *DirectBackend) AuthenticateMember(ctx context.Context, meta RequestMeta) (MemberSession, error) {
	identity, err := b.ops.authenticate(ctx, meta)
	if err != nil {
		return MemberSession{}, err
	}
	return NewMemberSession(identity), nil
}

// MemberSyncHeads 返回实时事件流所属成员的同步探针值。
func (b *DirectBackend) MemberSyncHeads(ctx context.Context, session MemberSession) (SyncHeads, error) {
	return b.ops.GetSyncHeads(ctx, RequestMeta{}, session.identity)
}

// AuthorizeAgentRunStream 校验运行过程流请求方对运行所属会话的阅读资格，并返回运行所属会话编号。
func (b *DirectBackend) AuthorizeAgentRunStream(ctx context.Context, meta RequestMeta, session MemberSession, runID string) (string, error) {
	conversationID, err := b.ops.authorizeAgentRunStream.Execute(ctx, session.identity, runID)
	if err != nil {
		return "", agentRunProcessError(ctx, meta, err, session.OrganizationID, runID)
	}
	return conversationID, nil
}

// SubscribeAgentRunStream 订阅本进程中该运行当前执行尝试的过程流，返回订阅时的快照与取消订阅函数；
// 运行不在本进程执行时返回 false，调用方按持久事实收敛。
func (b *DirectBackend) SubscribeAgentRunStream(runID string,
	onDelta func(agentruntime.StreamDelta), onEnd func()) (agentruntime.StreamSnapshot, func(), bool) {
	snapshot, subscription, running := b.ops.agentCoordinator.SubscribeRunStream(runID, onDelta, onEnd)
	if !running {
		return agentruntime.StreamSnapshot{}, nil, false
	}
	return snapshot, subscription.Close, true
}

// authenticateAccount 校验登录会话并返回当前账号。
func (g sessionGuard) authenticateAccount(ctx context.Context, meta RequestMeta) (*servermodels.AccountIdentity, error) {
	account, err := g.resolveAccount.Execute(ctx, meta.Token)
	if errors.Is(err, authaction.ErrIdentityNotFound) {
		return nil, g.loginRequired(ctx, meta)
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		slog.Warn("读取登录会话失败", "error", err)
		return nil, FailedError(meta, cervii18n.ErrorAuthenticationStatusFailed)
	}
	return account, nil
}

// authenticate 校验登录会话并返回账号在请求目标工作区中的成员身份。
func (g sessionGuard) authenticate(ctx context.Context, meta RequestMeta) (*servermodels.Identity, error) {
	account, err := g.authenticateAccount(ctx, meta)
	if err != nil {
		return nil, err
	}
	identity, err := authaction.ResolveMember(ctx, g.db, account, meta.WorkspaceID)
	if errors.Is(err, authaction.ErrMembershipNotFound) {
		slog.Info("账号不是目标工作区的有效成员", "account_id", account.Account.ID, "workspace_id", meta.WorkspaceID)
		return nil, SessionError(meta, SessionStateWorkspace, cervii18n.ErrorWorkspaceUnavailable)
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		slog.Warn("读取工作区成员身份失败", "account_id", account.Account.ID, "error", err)
		return nil, FailedError(meta, cervii18n.ErrorAuthenticationStatusFailed)
	}
	return identity, nil
}

// loginRequired 返回需要登录的会话错误；自托管部署尚未完成首次安装时返回初始化入口。
func (g sessionGuard) loginRequired(ctx context.Context, meta RequestMeta) error {
	if !g.deploymentMode.Managed() {
		installed, err := g.installationStatus.Execute(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("读取安装状态失败", "error", err)
			return FailedError(meta, cervii18n.ErrorInstallationStatusReadFailed)
		}
		if !installed {
			return SessionError(meta, SessionStateSetup, cervii18n.ErrorInstallationRequired)
		}
	}
	return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
}
