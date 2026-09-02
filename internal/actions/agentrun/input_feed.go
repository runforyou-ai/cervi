//go:build server

package agentrun

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

var errAgentRunSuppressed = errors.New("agent run suppressed")

type agentRunPolicyContext struct {
	ServiceSession *servermodels.ServiceSession
}

type agentRunPolicy interface {
	lockContext(context.Context, bun.IDB, *servermodels.AgentRun) (agentRunPolicyContext, error)
	prepareLocked(context.Context, bun.IDB, agentRunPolicyContext, *servermodels.AgentRun) (bool, error)
	loadMessages(context.Context, bun.IDB, *servermodels.AgentRun, int64) ([]agentruntime.Message, error)
	persistResponse(context.Context, bun.IDB, agentRunPolicyContext, *servermodels.AgentRun, string, string) error
	enqueueNext(context.Context, bun.IDB, agentRunPolicyContext, *servermodels.AgentRun, int64) error
}

type agentRunScope struct {
	TriggerType      domain.AgentTriggerType
	ServiceSessionID *string
}

type lockedAgentRun struct {
	PolicyContext agentRunPolicyContext
	State         *servermodels.ConversationAgentState
	Run           *servermodels.AgentRun
}

type databaseInputFeed struct {
	db        *bun.DB
	execution executionContext
	policy    agentRunPolicy
}

// Peek 返回尚未进入 TurnLoop 缓冲区的连续输入信号。
func (f *databaseInputFeed) Peek(ctx context.Context, afterSeq int64) ([]agentruntime.Trigger, error) {
	scope, err := agentRunScopeFor(&f.execution.Run)
	if err != nil {
		return nil, err
	}
	triggers := make([]agentruntime.Trigger, 0)
	query := f.db.NewSelect().TableExpr("conversation_agent_triggers AS cat").
		ColumnExpr("cat.trigger_seq AS seq").
		Join("JOIN conversation_agent_states AS cas ON cas.conversation_id = cat.conversation_id AND cas.organization_id = cat.organization_id AND cas.agent_identity_id = cat.agent_identity_id").
		Where("cat.organization_id = ?", f.execution.Run.OrganizationID).
		Where("cat.conversation_id = ?", f.execution.Run.ConversationID).
		Where("cat.agent_identity_id = ?", f.execution.Run.AgentIdentityID).
		Where("cat.trigger_seq > cas.processed_seq").
		Where("cat.trigger_seq > ?", afterSeq).
		OrderExpr("cat.trigger_seq ASC")
	scope.applySelect(query)
	if err := query.Scan(ctx, &triggers); err != nil {
		return nil, fmt.Errorf("peek agent triggers: %w", err)
	}
	return triggers, nil
}

// Claim 绑定当前所有已持久化输入，并按运行策略重建截至该边界的会话上下文。
func (f *databaseInputFeed) Claim(ctx context.Context, throughSeq int64) (agentruntime.ClaimedInput, error) {
	if throughSeq <= 0 {
		return agentruntime.ClaimedInput{}, errors.New("agent trigger sequence is invalid")
	}
	scope, err := agentRunScopeFor(&f.execution.Run)
	if err != nil {
		return agentruntime.ClaimedInput{}, err
	}
	var output agentruntime.ClaimedInput
	var previousEndSeq int64
	suppressed := false
	err = f.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		locked, err := lockAgentRun(ctx, tx, f.policy, &f.execution.Run)
		if err != nil {
			return fmt.Errorf("lock agent input: %w", err)
		}
		policyContext, state, run := locked.PolicyContext, locked.State, locked.Run
		if agentRunStatusTerminal(run.Status) {
			suppressed = true
			return nil
		}
		allowed, err := f.policy.prepareLocked(ctx, tx, policyContext, run)
		if err != nil {
			return err
		}
		if !allowed {
			suppressed = true
			return nil
		}
		if run.Status != string(domain.AgentRunStatusRunning) || state.DesiredSeq <= state.ProcessedSeq {
			return errors.New("agent run has no claimable input")
		}
		if run.TriggerEndSeq != nil {
			previousEndSeq = *run.TriggerEndSeq
		}
		claimEnd := min(throughSeq, state.DesiredSeq)
		if claimEnd <= state.ProcessedSeq || claimEnd < run.TriggerStartSeq {
			return errors.New("agent run input boundary is not claimable")
		}
		claimedSeqs, err := assignAgentTriggers(ctx, tx, run, scope, state.ProcessedSeq, claimEnd)
		if err != nil {
			return fmt.Errorf("claim agent triggers: %w", err)
		}
		if int64(len(claimedSeqs)) != claimEnd-state.ProcessedSeq {
			return errors.New("agent trigger sequence is not contiguous")
		}
		if _, err := tx.NewUpdate().Model(run).
			Set("trigger_start_seq = LEAST(trigger_start_seq, ?)", state.ProcessedSeq+1).
			Set("trigger_end_seq = ?", claimEnd).
			Set("updated_at = now()").
			WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("update agent run trigger boundary: %w", err)
		}
		messages, err := f.policy.loadMessages(ctx, tx, run, claimEnd)
		if err != nil {
			return err
		}
		output = agentruntime.ClaimedInput{Messages: messages, EndSeq: claimEnd}
		return nil
	})
	if err != nil {
		return agentruntime.ClaimedInput{}, err
	}
	if suppressed {
		return agentruntime.ClaimedInput{}, errAgentRunSuppressed
	}
	if scope.TriggerType == domain.AgentTriggerTypeCustomerAuto {
		slog.Info("网站客户 Agent 输入已认领",
			"agent_run_id", f.execution.Run.ID,
			"conversation_id", f.execution.Run.ConversationID,
			"service_session_id", *scope.ServiceSessionID,
			"trigger_start_seq", f.execution.Run.TriggerStartSeq,
			"previous_end_seq", previousEndSeq,
			"trigger_end_seq", output.EndSeq,
			"context_message_count", len(output.Messages),
		)
	}
	return output, nil
}

// agentRunScopeFor 从运行记录构造严格的 Trigger 查询范围。
func agentRunScopeFor(run *servermodels.AgentRun) (agentRunScope, error) {
	scope := agentRunScope{
		TriggerType:      domain.AgentTriggerType(run.TriggerType),
		ServiceSessionID: run.ServiceSessionID,
	}
	if err := validateAgentRunScope(scope.TriggerType, scope.ServiceSessionID); err != nil {
		return agentRunScope{}, err
	}
	return scope, nil
}

// validateAgentRunScope 校验运行类型和客服周期字段保持一致。
func validateAgentRunScope(triggerType domain.AgentTriggerType, serviceSessionID *string) error {
	switch triggerType {
	case domain.AgentTriggerTypeDirect:
		if serviceSessionID != nil {
			return errors.New("direct agent run cannot belong to a service session")
		}
	case domain.AgentTriggerTypeCustomerAuto:
		if serviceSessionID == nil {
			return errors.New("customer agent run requires a service session")
		}
	default:
		return fmt.Errorf("unsupported agent trigger type %q", triggerType)
	}
	return nil
}

// applySelect 把运行类型和客服周期加入 Trigger 查询。
func (s agentRunScope) applySelect(query *bun.SelectQuery) {
	query.Where("cat.trigger_type = ?", s.TriggerType).
		Where("cat.service_session_id IS NOT DISTINCT FROM ?", s.ServiceSessionID)
}

// lockAgentRun 按策略上下文、输入状态、运行记录的顺序取得事务锁。
func lockAgentRun(ctx context.Context, db bun.IDB, policy agentRunPolicy, initial *servermodels.AgentRun) (lockedAgentRun, error) {
	policyContext, err := policy.lockContext(ctx, db, initial)
	if err != nil {
		return lockedAgentRun{}, err
	}
	state := &servermodels.ConversationAgentState{}
	// 锁定一次运行对应的会话输入状态。
	if err := db.NewSelect().Model(state).
		Where("cas.conversation_id = ?", initial.ConversationID).
		Where("cas.organization_id = ?", initial.OrganizationID).
		Where("cas.agent_identity_id = ?", initial.AgentIdentityID).
		For("UPDATE").Scan(ctx); err != nil {
		return lockedAgentRun{}, err
	}
	run := &servermodels.AgentRun{}
	if err := db.NewSelect().Model(run).Where("agr.id = ?", initial.ID).For("UPDATE").Scan(ctx); err != nil {
		return lockedAgentRun{}, err
	}
	return lockedAgentRun{PolicyContext: policyContext, State: state, Run: run}, nil
}

// assignAgentTriggers 把连续范围内且属于当前策略的 Trigger 绑定到运行。
func assignAgentTriggers(ctx context.Context, db bun.IDB, run *servermodels.AgentRun, scope agentRunScope, afterSeq, throughSeq int64) ([]int64, error) {
	claimedSeqs := make([]int64, 0, throughSeq-afterSeq)
	err := db.NewRaw(`
		UPDATE conversation_agent_triggers
		SET agent_run_id = ?
		WHERE organization_id = ?
			AND conversation_id = ?
			AND agent_identity_id = ?
			AND trigger_type = ?
			AND service_session_id IS NOT DISTINCT FROM ?
			AND trigger_seq > ?
			AND trigger_seq <= ?
		RETURNING trigger_seq
	`, run.ID, run.OrganizationID, run.ConversationID, run.AgentIdentityID,
		scope.TriggerType, scope.ServiceSessionID, afterSeq, throughSeq,
	).Scan(ctx, &claimedSeqs)
	return claimedSeqs, err
}

type messageBoundary struct {
	OriginatedAt time.Time `bun:"originated_at"`
	SourceOrder  int64     `bun:"source_order"`
	ID           string    `bun:"id"`
}

// loadClaimedMessageBoundary 读取一次已认领输入对应的稳定消息边界。
func loadClaimedMessageBoundary(ctx context.Context, db bun.IDB, run *servermodels.AgentRun, endSeq int64) (messageBoundary, error) {
	scope, err := agentRunScopeFor(run)
	if err != nil {
		return messageBoundary{}, err
	}
	boundary := messageBoundary{}
	query := db.NewSelect().TableExpr("conversation_agent_triggers AS cat").
		ColumnExpr("msg.originated_at, msg.source_order, msg.id").
		Join("JOIN messages AS msg ON msg.id = cat.trigger_message_id AND msg.organization_id = cat.organization_id AND msg.conversation_id = cat.conversation_id").
		Where("cat.organization_id = ?", run.OrganizationID).
		Where("cat.conversation_id = ?", run.ConversationID).
		Where("cat.agent_identity_id = ?", run.AgentIdentityID).
		Where("cat.trigger_seq = ?", endSeq)
	scope.applySelect(query)
	if err := query.Scan(ctx, &boundary); err != nil {
		return messageBoundary{}, fmt.Errorf("load claimed input boundary: %w", err)
	}
	return boundary, nil
}

type claimedMessageRow struct {
	Body           string `bun:"body"`
	SenderSourceID string `bun:"sender_source_id"`
}

// loadClaimedConversationMessages 读取不越过已认领 Trigger 的最近会话上下文。
func loadClaimedConversationMessages(ctx context.Context, db bun.IDB, run *servermodels.AgentRun, endSeq int64) ([]agentruntime.Message, error) {
	boundary, err := loadClaimedMessageBoundary(ctx, db, run, endSeq)
	if err != nil {
		return nil, err
	}
	rows := make([]claimedMessageRow, 0, agentHistoryLimit)
	if err := db.NewSelect().TableExpr("messages AS msg").
		ColumnExpr("msg.body").
		ColumnExpr("cs.source_id AS sender_source_id").
		Join("JOIN conversation_participants AS cp ON cp.id = msg.sender_participant_id AND cp.organization_id = msg.organization_id AND cp.conversation_id = msg.conversation_id").
		Join("JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Where("msg.organization_id = ?", run.OrganizationID).
		Where("msg.conversation_id = ?", run.ConversationID).
		Where("msg.type = ?", domain.MessageTypeText).
		Where("msg.deleted_at IS NULL").
		Where("(msg.originated_at, msg.source_order, msg.id) <= (?, ?, ?)", boundary.OriginatedAt, boundary.SourceOrder, boundary.ID).
		OrderExpr("msg.originated_at DESC, msg.source_order DESC, msg.id DESC").
		Limit(agentHistoryLimit).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("load claimed conversation context: %w", err)
	}
	slices.Reverse(rows)
	messages := make([]agentruntime.Message, 0, len(rows))
	for _, row := range rows {
		role := agentruntime.MessageRoleUser
		if row.SenderSourceID == run.AgentIdentityID {
			role = agentruntime.MessageRoleAssistant
		}
		messages = append(messages, agentruntime.Message{Role: role, Content: row.Body})
	}
	return messages, nil
}
