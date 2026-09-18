//go:build server

package agentrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	deliveryaction "github.com/runforyou-ai/cervi/internal/actions/customerdelivery"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// handoffReasonTextMaxRunes 限制写入转人工事件的原因说明长度。
const handoffReasonTextMaxRunes = 500

// customerHandoff 描述一次把客服处理周期从 AI 员工交给人工的事实。
type customerHandoff struct {
	PolicyContext   agentRunPolicyContext
	AgentIdentityID string
	Route           chatstate.RouteSnapshot
	NoticeKey       string // 对客通知的幂等键。
	EventKey        string // 转人工系统事件的幂等键。
	Notice          string
	Reason          domain.AgentHandoffReason
	ReasonText      string
	AgentRunID      *string
}

// applyCustomerHandoff 在调用方持有会话锁的事务中写入转人工事件与对客通知，按去向更新负责人与团队，并推进会话版本。
func applyCustomerHandoff(ctx context.Context, db bun.IDB, enqueuer servertask.TxEnqueuer, handoff customerHandoff) (*servermodels.Message, error) {
	session := handoff.PolicyContext.ServiceSession
	participantID, err := ensureCustomerAgentParticipant(ctx, db, session.OrganizationID, session.ConversationID, handoff.AgentIdentityID)
	if err != nil {
		return nil, err
	}
	var agentName string
	if err := db.NewSelect().Model((*servermodels.OrganizationIdentity)(nil)).Column("display_name").
		Where("oi.organization_id = ? AND oi.id = ?", session.OrganizationID, handoff.AgentIdentityID).
		Scan(ctx, &agentName); err != nil {
		return nil, fmt.Errorf("load handoff agent name: %w", err)
	}
	// 原因说明截断到固定长度，只进入成员可见的系统事件。
	reasonText := []rune(strings.TrimSpace(handoff.ReasonText))
	if len(reasonText) > handoffReasonTextMaxRunes {
		reasonText = reasonText[:handoffReasonTextMaxRunes]
	}
	payload, err := json.Marshal(domain.ServiceSessionHandedOffEvent{
		ServiceSessionID: session.ID, FromIdentityID: handoff.AgentIdentityID, FromDisplayName: agentName,
		Target: handoff.Route.Target(), Reason: handoff.Reason, ReasonText: string(reasonText), AgentRunID: handoff.AgentRunID,
	})
	if err != nil {
		return nil, fmt.Errorf("encode service session handoff event: %w", err)
	}
	eventType := string(domain.ConversationSystemEventServiceSessionHandedOff)
	// 系统事件先于对客通知写入，会话最后消息保持为对客文本。
	if _, _, err := appendAgentMessage(ctx, db, handoff.PolicyContext.Conversation, &servermodels.Message{
		ID: uuid.NewV7().String(), OrganizationID: session.OrganizationID, ConversationID: session.ConversationID,
		ServiceSessionID: &session.ID, Type: string(domain.MessageTypeSystem), Visibility: string(domain.MessageVisibilityInternalOnly),
		SystemEventType: &eventType, SystemEventPayload: payload, IdempotencyKey: &handoff.EventKey,
	}); err != nil {
		return nil, fmt.Errorf("append service session handoff event: %w", err)
	}
	notice, err := appendCustomerAgentMessage(ctx, db, enqueuer, handoff.PolicyContext, &servermodels.Message{
		ID: uuid.NewV7().String(), OrganizationID: session.OrganizationID, ConversationID: session.ConversationID,
		ServiceSessionID: &session.ID, SenderParticipantID: &participantID,
		Type: string(domain.MessageTypeText), Body: handoff.Notice, IdempotencyKey: &handoff.NoticeKey,
	})
	if err != nil {
		return nil, fmt.Errorf("append customer handoff notice: %w", err)
	}
	update := db.NewUpdate().Model(session).
		Set("assignee_identity_id = ?", handoff.Route.AssigneeIdentityID).
		Set("team_id = ?", handoff.Route.TeamID).
		Set("updated_at = now()").
		WherePK().Where("organization_id = ?", session.OrganizationID)
	if handoff.Route.AssigneeIdentityID != nil {
		update = update.Set("assigned_at = COALESCE(assigned_at, ?)", time.Now().UTC())
	}
	if _, err := update.Exec(ctx); err != nil {
		return nil, fmt.Errorf("hand off service session: %w", err)
	}
	session.AssigneeIdentityID, session.TeamID = handoff.Route.AssigneeIdentityID, handoff.Route.TeamID
	if err := chatstate.TouchConversation(ctx, db, handoff.PolicyContext.Conversation); err != nil {
		return nil, err
	}
	slog.Info("客户会话已由 AI 员工转交人工",
		"organization_id", session.OrganizationID, "conversation_id", session.ConversationID,
		"service_session_id", session.ID, "agent_identity_id", handoff.AgentIdentityID,
		"target_kind", handoff.Route.Target().Kind, "reason", handoff.Reason)
	return notice, nil
}

// resolveCustomerHandoffRoute 在进入会话锁之前读取客户会话所属渠道并解析转人工去向，目标身份取 FOR KEY SHARE。
func resolveCustomerHandoffRoute(ctx context.Context, db bun.IDB, organizationID, conversationID string) (*servermodels.Channel, chatstate.RouteSnapshot, error) {
	channel, err := chatstate.LoadConversationChannel(ctx, db, organizationID, conversationID)
	if err != nil {
		return nil, chatstate.RouteSnapshot{}, err
	}
	route, err := chatstate.ResolveHandoffRoute(ctx, db, channel, true)
	return channel, route, err
}

// customerHandoffAllowed 复核运行仍持有当前开放周期的处理权；转人工不要求 AI 员工仍满足继续执行的资格。
func customerHandoffAllowed(session *servermodels.ServiceSession, run *servermodels.AgentRun) bool {
	return run.ScopeID == session.ID &&
		domain.ServiceSessionStatus(session.Status) == domain.ServiceSessionStatusOpen &&
		session.AssigneeIdentityID != nil && *session.AssigneeIdentityID == run.AgentIdentityID
}

// settleHandoffLane 把原 AI 员工的输入队列结算到锁内 desired_seq，返回结算边界；未认领的输入由人工处理。
func settleHandoffLane(ctx context.Context, db bun.IDB, lane *servermodels.AgentLane) (int64, error) {
	if _, err := db.NewUpdate().Model(lane).
		Set("processed_seq = desired_seq").
		Set("updated_at = now()").
		WherePK().Exec(ctx); err != nil {
		return 0, fmt.Errorf("settle handed off agent lane: %w", err)
	}
	return lane.DesiredSeq, nil
}

// completeCustomerHandoff 在同一事务内提交模型或 Runtime 给出的转人工决定：写入事件与通知、改派负责人、结算输入队列并结束运行。
func (a *ExecuteAction) completeCustomerHandoff(ctx context.Context, execution executionContext, policy agentRunPolicy, result agentruntime.RunResult, usage []byte, blocks []servermodels.AgentRunBlock) error {
	notice := strings.TrimSpace(result.Content)
	suppressed, completed := false, false
	err := realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		channel, route, err := resolveCustomerHandoffRoute(ctx, tx, execution.Run.OrganizationID, execution.Run.ConversationID)
		if err != nil {
			return err
		}
		locked, err := lockAgentRun(ctx, tx, policy, &execution.Run)
		if err != nil {
			return fmt.Errorf("lock agent run for handoff: %w", err)
		}
		policyContext, lane, run := locked.PolicyContext, locked.Lane, locked.Run
		if agentRunStatusTerminal(run.Status) {
			return nil
		}
		if !customerHandoffAllowed(policyContext.ServiceSession, run) {
			suppressed = true
			if err := suppressCustomerRun(ctx, tx, run, policyContext.ServiceSession); err != nil {
				return err
			}
			if err := chatstate.TouchConversation(ctx, tx, policyContext.Conversation); err != nil {
				return err
			}
			return scheduleNextRun(ctx, tx, a.enqueuer, policy, policyContext, run.OrganizationID, domain.AgentExecutionScopeKind(run.ScopeKind), run.ScopeID)
		}
		if run.Status != string(domain.AgentRunStatusRunning) || run.InputEndSeq == nil ||
			*run.InputEndSeq != result.EndSeq || run.InputStartSeq != lane.ProcessedSeq+1 {
			return errors.New("agent run handoff boundary is inconsistent")
		}
		if notice == "" {
			notice, _ = cervii18n.Localize(channel.DefaultLocale, cervii18n.AgentCustomerHandoffFallback)
		}
		message, err := applyCustomerHandoff(ctx, tx, a.enqueuer, customerHandoff{
			PolicyContext: policyContext, AgentIdentityID: run.AgentIdentityID, Route: route,
			NoticeKey: "agent:" + run.ID, EventKey: "agent:" + run.ID + ":handoff-event", Notice: notice,
			Reason: result.Decision.Reason, ReasonText: result.Decision.ReasonText, AgentRunID: &run.ID,
		})
		if err != nil {
			return err
		}
		if len(blocks) > 0 {
			if _, err := tx.NewInsert().Model(&blocks).Exec(ctx); err != nil {
				return fmt.Errorf("persist agent run blocks: %w", err)
			}
		}
		settledSeq, err := settleHandoffLane(ctx, tx, lane)
		if err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model(run).
			Set("status = ?", domain.AgentRunStatusSucceeded).
			Set("outcome = ?", domain.AgentRunOutcomeHandoff).
			Set("outcome_reason = ?", result.Decision.Reason).
			Set("response_message_id = ?", message.ID).
			Set("handoff_settled_seq = ?", settledSeq).
			Set("usage = ?::jsonb", string(usage)).
			Set("last_error = NULL").
			Set("error_code = NULL").
			Set("completed_at = now()").
			Set("updated_at = now()").
			WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("complete handed off agent run: %w", err)
		}
		completed = true
		return nil
	})
	if err != nil {
		return err
	}
	if suppressed {
		slog.Warn("客户 Agent 迟到的转人工结果已抑制", "agent_run_id", execution.Run.ID, "conversation_id", execution.Run.ConversationID)
	}
	if completed {
		slog.Info("客户 Agent 转交人工", "agent_run_id", execution.Run.ID, "conversation_id", execution.Run.ConversationID,
			"service_session_id", execution.Run.ScopeID, "reason", result.Decision.Reason)
	}
	return nil
}

// failCustomerRun 把客服运行失败收敛为转人工：绑定失败输入，写入内部错误消息、转人工事件与对客通知，改派负责人并结算输入队列。
func (a *ExecuteAction) failCustomerRun(ctx context.Context, initial *servermodels.AgentRun, policy agentRunPolicy, lastError string, reason domain.AgentHandoffReason) (bool, error) {
	terminal := false
	err := realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		channel, route, err := resolveCustomerHandoffRoute(ctx, tx, initial.OrganizationID, initial.ConversationID)
		if err != nil {
			return err
		}
		locked, err := lockAgentRun(ctx, tx, policy, initial)
		if err != nil {
			return fmt.Errorf("lock agent run for failure: %w", err)
		}
		policyContext, lane, run := locked.PolicyContext, locked.Lane, locked.Run
		if agentRunStatusTerminal(run.Status) {
			terminal = true
			return nil
		}
		if !customerHandoffAllowed(policyContext.ServiceSession, run) {
			terminal = true
			if err := suppressCustomerRun(ctx, tx, run, policyContext.ServiceSession); err != nil {
				return err
			}
			if err := chatstate.TouchConversation(ctx, tx, policyContext.Conversation); err != nil {
				return err
			}
			return scheduleNextRun(ctx, tx, a.enqueuer, policy, policyContext, run.OrganizationID, domain.AgentExecutionScopeKind(run.ScopeKind), run.ScopeID)
		}
		if run.Status != string(domain.AgentRunStatusQueued) && run.Status != string(domain.AgentRunStatusRunning) {
			return fmt.Errorf("cannot fail agent run in status %q", run.Status)
		}
		failureEnd := run.InputStartSeq
		if run.InputEndSeq != nil {
			failureEnd = *run.InputEndSeq
		}
		if run.InputStartSeq != lane.ProcessedSeq+1 || failureEnd < run.InputStartSeq || failureEnd > lane.DesiredSeq {
			return errors.New("agent run failure boundary is inconsistent")
		}
		failedSeqs, err := claimLaneInputs(ctx, tx, run, lane.ProcessedSeq, failureEnd)
		if err != nil {
			return err
		}
		if int64(len(failedSeqs)) != failureEnd-lane.ProcessedSeq {
			return errors.New("failed agent input sequence is not contiguous")
		}
		participantID, err := ensureCustomerAgentParticipant(ctx, tx, run.OrganizationID, run.ConversationID, run.AgentIdentityID)
		if err != nil {
			return err
		}
		// 运行错误只作为成员可见的内部消息写入，不投递、不计入首响。
		errorKey := "agent:" + run.ID + ":error"
		if _, _, err := appendAgentMessage(ctx, tx, policyContext.Conversation, &servermodels.Message{
			ID: uuid.NewV7().String(), OrganizationID: run.OrganizationID, ConversationID: run.ConversationID,
			ServiceSessionID: &policyContext.ServiceSession.ID, SenderParticipantID: &participantID,
			Type: string(domain.MessageTypeAgentError), Visibility: string(domain.MessageVisibilityInternalOnly), IdempotencyKey: &errorKey,
		}); err != nil {
			return err
		}
		notice, _ := cervii18n.Localize(channel.DefaultLocale, cervii18n.AgentCustomerFailureFallback)
		message, err := applyCustomerHandoff(ctx, tx, a.enqueuer, customerHandoff{
			PolicyContext: policyContext, AgentIdentityID: run.AgentIdentityID, Route: route,
			NoticeKey: "agent:" + run.ID, EventKey: "agent:" + run.ID + ":handoff-event", Notice: notice,
			Reason: reason, ReasonText: lastError, AgentRunID: &run.ID,
		})
		if err != nil {
			return err
		}
		settledSeq, err := settleHandoffLane(ctx, tx, lane)
		if err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model(run).
			Set("status = ?", domain.AgentRunStatusFailed).
			Set("outcome = ?", domain.AgentRunOutcomeHandoff).
			Set("outcome_reason = ?", reason).
			Set("response_message_id = ?", message.ID).
			Set("input_end_seq = ?", failureEnd).
			Set("handoff_settled_seq = ?", settledSeq).
			Set("last_error = ?", lastError).
			Set("error_code = NULL").
			Set("completed_at = now()").
			Set("updated_at = now()").
			WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("fail handed off agent run: %w", err)
		}
		return nil
	})
	return terminal, err
}

// HandOffAgentServiceSessions 在管理操作事务中把 AI 员工仍负责的开放客服周期交给人工：取消在途运行并结算输入队列，写入事件与对客通知并改派负责人。
// 调用方已对该 AI 员工身份取 FOR UPDATE；返回被取消的运行编号，调用方在提交后中断本进程中的模型调用。
func (a *ExecuteAction) HandOffAgentServiceSessions(ctx context.Context, db bun.IDB, organizationID, agentIdentityID, operationID string) ([]string, error) {
	var sessions []struct {
		ID             string `bun:"id"`
		ConversationID string `bun:"conversation_id"`
	}
	if err := db.NewSelect().Model((*servermodels.ServiceSession)(nil)).
		Column("ss.id", "ss.conversation_id").
		Where("ss.organization_id = ? AND ss.assignee_identity_id = ? AND ss.status = ?", organizationID, agentIdentityID, domain.ServiceSessionStatusOpen).
		OrderExpr("ss.conversation_id").
		Scan(ctx, &sessions); err != nil {
		return nil, fmt.Errorf("load agent open service sessions: %w", err)
	}
	cancelled := make([]string, 0)
	for _, row := range sessions {
		runIDs, err := handOffUnavailableAgentSession(ctx, db, a.enqueuer, organizationID, row.ConversationID, row.ID, agentIdentityID, "handoff:"+row.ID+":"+operationID, true)
		if err != nil {
			return nil, err
		}
		cancelled = append(cancelled, runIDs...)
	}
	return cancelled, nil
}

// handOffUnavailableAgentSession 把失去接客资格的 AI 员工负责的指定周期交给人工，周期已变化时跳过。
// lock 为 true 时按转交目标身份、渠道身份、会话的锁序加锁；调用方已持有会话锁时传 false，目标身份与外发目标只读取不加锁。
func handOffUnavailableAgentSession(ctx context.Context, db bun.IDB, enqueuer servertask.TxEnqueuer, organizationID, conversationID, serviceSessionID, agentIdentityID, key string, lock bool) ([]string, error) {
	channel, err := chatstate.LoadConversationChannel(ctx, db, organizationID, conversationID)
	if err != nil {
		return nil, err
	}
	route, err := chatstate.ResolveHandoffRoute(ctx, db, channel, lock)
	if err != nil {
		return nil, err
	}
	loadDeliveryRoute := deliveryaction.LoadRoute
	if lock {
		loadDeliveryRoute = deliveryaction.Prepare
	}
	deliveryRoute, err := loadDeliveryRoute(ctx, db, organizationID, conversationID)
	if err != nil {
		return nil, err
	}
	conversation, session, err := chatstate.LockCustomerServiceSession(ctx, db, organizationID, conversationID)
	if err != nil {
		return nil, err
	}
	if session.ID != serviceSessionID || domain.ServiceSessionStatus(session.Status) != domain.ServiceSessionStatusOpen ||
		session.AssigneeIdentityID == nil || *session.AssigneeIdentityID != agentIdentityID {
		return nil, nil
	}
	runIDs, err := cancelServiceSessionRuns(ctx, db, organizationID, session.ID, agentIdentityID, domain.AgentRunErrorCodeAgentUnavailable)
	if err != nil {
		return nil, err
	}
	notice, _ := cervii18n.Localize(channel.DefaultLocale, cervii18n.AgentCustomerFailureFallback)
	if _, err := applyCustomerHandoff(ctx, db, enqueuer, customerHandoff{
		PolicyContext:   agentRunPolicyContext{Conversation: conversation, ServiceSession: session, DeliveryRoute: deliveryRoute},
		AgentIdentityID: agentIdentityID, Route: route,
		NoticeKey: key, EventKey: key + ":event", Notice: notice, Reason: domain.AgentHandoffReasonAgentUnavailable,
	}); err != nil {
		return nil, err
	}
	return runIDs, nil
}
