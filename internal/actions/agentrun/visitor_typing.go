//go:build server

package agentrun

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// visitorTypingRefreshInterval 是运行期间刷新访客输入状态的间隔，短于访客端的到期清除时间。
const visitorTypingRefreshInterval = 3 * time.Second

// startVisitorTyping 在网站客户会话的运行期间向访客发布「正在回复」并按间隔刷新，返回发布停止状态的收尾函数；
// 其他执行范围与非网站渠道不发布，返回的收尾函数无副作用。
func (a *ExecuteAction) startVisitorTyping(ctx context.Context, run *servermodels.AgentRun) func() {
	if domain.AgentExecutionScopeKind(run.ScopeKind) != domain.AgentExecutionScopeServiceSession {
		return func() {}
	}
	var channelIdentityID string
	err := a.db.NewSelect().TableExpr("customer_conversations AS cc").
		ColumnExpr("cci.id").
		Join("JOIN contact_channel_identities AS cci ON cci.organization_id = cc.organization_id AND cci.id = cc.contact_channel_identity_id").
		Join("JOIN channels AS c ON c.organization_id = cci.organization_id AND c.id = cci.channel_id AND c.type = ?", domain.ChannelTypeWebsite).
		Where("cc.organization_id = ? AND cc.conversation_id = ?", run.OrganizationID, run.ConversationID).
		Scan(ctx, &channelIdentityID)
	if errors.Is(err, sql.ErrNoRows) {
		return func() {}
	}
	if err != nil {
		slog.Warn("读取访客输入状态受众失败", "organization_id", run.OrganizationID, "conversation_id", run.ConversationID, "agent_run_id", run.ID, "error", err)
		return func() {}
	}
	publish := func(active bool) {
		realtime.Publish(realtime.VisitorDirectoryTyping(run.OrganizationID, channelIdentityID, run.ConversationID, active))
	}
	publish(true)
	stop := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(visitorTypingRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				publish(true)
			}
		}
	}()
	return func() {
		close(stop)
		<-stopped
		publish(false)
	}
}
