//go:build server

package agentrun

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// runTypingRefreshInterval 是运行期间刷新输入状态的间隔，短于各端的到期清除时间。
const runTypingRefreshInterval = 3 * time.Second

// runTypingNotifications 按运行会话的当前受众构造 AI 员工输入状态通知：服务周期发往企业客服共享受众，网站渠道同时发往访客；非渠道来源的服务周期与其余会话发往在场的在职真人成员。
func (a *ExecuteAction) runTypingNotifications(ctx context.Context, run *servermodels.AgentRun, active bool) ([]realtime.Notification, error) {
	var agentSubjectID string
	if err := a.db.NewSelect().TableExpr("chat_subjects AS cs").
		Column("cs.id").
		Where("cs.organization_id = ? AND cs.kind = ? AND cs.source_id = ?", run.OrganizationID, domain.ChatSubjectKindOrganizationIdentity, run.AgentIdentityID).
		Scan(ctx, &agentSubjectID); err != nil {
		return nil, fmt.Errorf("load agent typing sender: %w", err)
	}
	notifications := make([]realtime.Notification, 0, 2)
	if domain.AgentExecutionScopeKind(run.ScopeKind) == domain.AgentExecutionScopeServiceSession {
		notifications = append(notifications, realtime.ServiceInboxConversationTyping(run.OrganizationID, run.ConversationID, agentSubjectID, active))
		var channel struct {
			IdentityID *string `bun:"identity_id"`
		}
		err := a.db.NewSelect().TableExpr("channel_conversations AS cc").
			ColumnExpr("CASE WHEN c.type = ? THEN cci.id END AS identity_id", domain.ChannelTypeWebsite).
			Join("JOIN contact_channel_identities AS cci ON cci.organization_id = cc.organization_id AND cci.id = cc.contact_channel_identity_id").
			Join("JOIN channels AS c ON c.organization_id = cci.organization_id AND c.id = cci.channel_id").
			Where("cc.organization_id = ? AND cc.conversation_id = ?", run.OrganizationID, run.ConversationID).
			Scan(ctx, &channel)
		// 渠道来源只另外通知网站访客，其他来源继续通知会话中的真人成员。
		if err == nil {
			if channel.IdentityID != nil {
				notifications = append(notifications, realtime.VisitorDirectoryTyping(run.OrganizationID, *channel.IdentityID, run.ConversationID, active))
			}
			return notifications, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("load visitor typing audience: %w", err)
		}
	}
	var userIDs []string
	if err := a.db.NewSelect().TableExpr("conversation_participants AS cp").
		Join("JOIN chat_subjects AS cs ON cs.organization_id = cp.organization_id AND cs.id = cp.subject_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("JOIN users AS u ON u.organization_id = cs.organization_id AND u.identity_id = cs.source_id").
		Column("u.id").
		Where("cp.organization_id = ? AND cp.conversation_id = ? AND cp.left_at IS NULL", run.OrganizationID, run.ConversationID).
		Where("u.status = ?", domain.UserStatusActive).
		Scan(ctx, &userIDs); err != nil {
		return nil, fmt.Errorf("load member typing audience: %w", err)
	}
	for _, userID := range userIDs {
		notifications = append(notifications, realtime.UserConversationTyping(run.OrganizationID, userID, run.ConversationID, agentSubjectID, active))
	}
	return notifications, nil
}

// publishRunTyping 按当前受众发布一次 AI 员工输入状态，读取受众失败时只记录日志。
func (a *ExecuteAction) publishRunTyping(ctx context.Context, run *servermodels.AgentRun, active bool) {
	notifications, err := a.runTypingNotifications(ctx, run, active)
	if err != nil {
		slog.Warn("读取 AI 员工输入状态受众失败", "organization_id", run.OrganizationID, "agent_run_id", run.ID, "error", err)
		return
	}
	realtime.Publish(notifications...)
}

// runTyping 在运行期间按间隔发布正在输入，截止时间到达或停止时发布一次停止输入。
type runTyping struct {
	mu       sync.Mutex
	deadline time.Time
	ended    bool
	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}
}

// startRunTyping 立即开始按间隔发布；deadline 为零值表示持续到停止。
func startRunTyping(ctx context.Context, publish func(active bool), deadline time.Time, interval time.Duration) *runTyping {
	typing := &runTyping{deadline: deadline, stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(typing.done)
		defer publish(false)
		publish(true)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-typing.stop:
				return
			case <-ctx.Done():
				typing.end()
				return
			case now := <-ticker.C:
				// 截止判断与续期在同一把锁内完成。
				typing.mu.Lock()
				if !typing.deadline.IsZero() && !now.Before(typing.deadline) {
					typing.ended = true
				}
				ended := typing.ended
				typing.mu.Unlock()
				if ended {
					return
				}
				publish(true)
			}
		}
	}()
	return typing
}

// extend 把截止时间推迟到指定时刻，发布已结束时返回 false。
func (t *runTyping) extend(deadline time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ended {
		return false
	}
	t.deadline = deadline
	return true
}

// end 标记发布结束，此后 extend 返回 false。
func (t *runTyping) end() {
	t.mu.Lock()
	t.ended = true
	t.mu.Unlock()
}

// close 停止发布并等待停止输入发出；可并发、重复调用。
func (t *runTyping) close() {
	t.end()
	t.stopOnce.Do(func() { close(t.stop) })
	<-t.done
}

// startServerRunTyping 在服务端执行运行期间发布 AI 员工正在输入，返回停止发布的收尾函数。
func (a *ExecuteAction) startServerRunTyping(ctx context.Context, run *servermodels.AgentRun) func() {
	publishCtx := context.WithoutCancel(ctx)
	return startRunTyping(ctx, func(active bool) { a.publishRunTyping(publishCtx, run, active) }, time.Time{}, runTypingRefreshInterval).close
}

// holdDeviceRunTyping 在设备持有租约期间发布 AI 员工正在输入，截止到租约到期；已在发布时只推迟截止时间，运行已不在进行时不发布。
func (a *ExecuteAction) holdDeviceRunTyping(ctx context.Context, run *servermodels.AgentRun, leaseExpiresAt time.Time) {
	a.typingMu.Lock()
	defer a.typingMu.Unlock()
	// 持锁确认运行仍持有租约：收尾先提交时这里读到终态，收尾后提交时由收尾释放本次发布。
	holding, err := a.db.NewSelect().Model((*servermodels.AgentRun)(nil)).
		Where("agr.id = ? AND agr.status = ? AND agr.lease_expires_at > clock_timestamp()", run.ID, domain.AgentRunStatusRunning).
		Exists(ctx)
	if err != nil {
		slog.Warn("读取设备运行状态失败", "organization_id", run.OrganizationID, "agent_run_id", run.ID, "error", err)
		return
	}
	current := a.deviceTyping[run.ID]
	if !holding {
		if current != nil {
			current.close()
		}
		return
	}
	if current != nil {
		if current.extend(leaseExpiresAt) {
			return
		}
		// 旧发布器已判定结束，等它发出停止输入后再开始新的发布。
		current.close()
	}
	publishCtx := context.WithoutCancel(ctx)
	typing := startRunTyping(publishCtx, func(active bool) { a.publishRunTyping(publishCtx, run, active) }, leaseExpiresAt, runTypingRefreshInterval)
	a.deviceTyping[run.ID] = typing
	go func() {
		<-typing.done
		a.typingMu.Lock()
		if a.deviceTyping[run.ID] == typing {
			delete(a.deviceTyping, run.ID)
		}
		a.typingMu.Unlock()
	}()
}

// releaseDeviceRunTyping 结束设备运行的输入状态发布。
func (a *ExecuteAction) releaseDeviceRunTyping(runID string) {
	a.typingMu.Lock()
	typing := a.deviceTyping[runID]
	a.typingMu.Unlock()
	if typing != nil {
		typing.close()
	}
}
