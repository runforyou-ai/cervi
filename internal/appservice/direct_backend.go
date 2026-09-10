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
	knowledgebaseaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	mcpserveraction "github.com/runforyou-ai/cervi/internal/actions/mcpserver"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeprocessing"
	mcpintegration "github.com/runforyou-ai/cervi/internal/integration/mcp"
	"github.com/runforyou-ai/cervi/internal/integration/modelprovider"
	"github.com/runforyou-ai/cervi/internal/integration/telegram"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/runforyou-ai/cervi/internal/tenant"
	"github.com/uptrace/bun"
)

var (
	_ Backend            = (*DirectBackend)(nil)
	_ WorkspaceInstaller = (*DirectBackend)(nil)
)

// sessionGuard 解析请求所属企业并校验登录令牌。
type sessionGuard struct {
	resolveTenant   tenant.Resolver
	resolveIdentity *authaction.ResolveIdentityQuery
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
	agentOps
	knowledgeOps
	integrationOps
	fileOps
}

// NewDirectBackend 创建直接访问服务端存储的应用后端。
func NewDirectBackend(db *bun.DB, localFiles *serverfilecontent.LocalStore, tenantResolver tenant.Resolver, agentScheduler conversationaction.AgentMessageScheduler, agentCoordinator *agentrunaction.ExecuteAction, taskEnqueuer servertask.TxEnqueuer, knowledgeProcessor *knowledgeprocessing.Client) *DirectBackend {
	connectionRunner := connectiontest.NewRunner(10 * time.Second)
	connectionClient := connectiontest.NewHTTPClient()
	modelProviderRegistry := modelprovider.NewRegistry(connectionClient)
	telegramAPI := telegram.NewClient(connectionClient)
	mcpTest := mcpserveraction.NewTestConnectionAction(mcpintegration.NewClient())
	mcpScheduler := mcpserveraction.NewToolsScheduler(taskEnqueuer)
	guard := sessionGuard{resolveTenant: tenantResolver, resolveIdentity: authaction.NewResolveIdentityQuery(db)}
	documentQuery := knowledgebaseaction.NewDocumentQuery(db)
	documentQuery.SetSegmentReader(knowledgeProcessor)
	ops := &directOperations{
		sessionGuard:    guard,
		authOps:         newAuthOps(db),
		conversationOps: newConversationOps(db, agentScheduler, agentCoordinator, taskEnqueuer),
		inboxOps:        newInboxOps(db, taskEnqueuer),
		channelOps:      newChannelOps(db, connectionRunner, telegramAPI),
		contactOps:      newContactOps(db),
		directoryOps:    newDirectoryOps(db),
		agentOps:        newAgentOps(db, agentCoordinator),
		knowledgeOps:    newKnowledgeOps(db, taskEnqueuer, documentQuery, knowledgeProcessor),
		integrationOps:  newIntegrationOps(db, connectionRunner, modelProviderRegistry, mcpTest, mcpScheduler),
		fileOps:         newFileOps(db, connectionRunner, localFiles),
	}
	return &DirectBackend{ops: ops}
}

// InstallWorkspace 创建企业管理员并返回登录令牌。
func (b *DirectBackend) InstallWorkspace(ctx context.Context, meta RequestMeta, input InstallWorkspaceInput) (Auth, error) {
	return b.ops.InstallWorkspace(ctx, meta, input)
}

// requireInitialized 解析当前请求的企业范围，并校验该企业是否已完成初始化。
func (g sessionGuard) requireInitialized(ctx context.Context, meta RequestMeta) (tenant.Scope, error) {
	scope, err := g.resolveTenant.Resolve(ctx, tenant.AccessHost(ctx))
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
func (g sessionGuard) authenticate(ctx context.Context, meta RequestMeta) (*servermodels.Identity, error) {
	scope, err := g.requireInitialized(ctx, meta)
	if err != nil {
		return nil, err
	}
	if meta.Token == "" {
		return nil, SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	identity, err := g.resolveIdentity.Execute(ctx, scope.OrganizationID, meta.Token)
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
