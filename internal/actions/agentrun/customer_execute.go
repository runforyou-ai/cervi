//go:build server

package agentrun

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

var errCustomerRunSuppressed = errors.New("customer agent run suppressed")

type customerInputFeed struct {
	db        *bun.DB
	execution executionContext
}

// Peek 返回当前客户 Agent Run 尚未进入 TurnLoop 的连续输入信号。
func (f *customerInputFeed) Peek(ctx context.Context, afterSeq int64) ([]agentruntime.Trigger, error) {
	if f.execution.Run.ServiceSessionID == nil {
		return nil, errCustomerRunSuppressed
	}
	triggers := make([]agentruntime.Trigger, 0)
	err := f.db.NewSelect().
		TableExpr("conversation_agent_triggers AS cat").
		ColumnExpr("cat.trigger_seq AS seq").
		Join("JOIN conversation_agent_states AS cas ON cas.conversation_id = cat.conversation_id AND cas.organization_id = cat.organization_id AND cas.agent_identity_id = cat.agent_identity_id").
		Where("cat.organization_id = ?", f.execution.Run.OrganizationID).
		Where("cat.conversation_id = ?", f.execution.Run.ConversationID).
		Where("cat.agent_identity_id = ?", f.execution.Run.AgentIdentityID).
		Where("cat.trigger_type = ?", domain.AgentTriggerTypeCustomerAuto).
		Where("cat.service_session_id = ?", *f.execution.Run.ServiceSessionID).
		Where("cat.trigger_seq > cas.processed_seq").
		Where("cat.trigger_seq > ?", afterSeq).
		OrderExpr("cat.trigger_seq ASC").
		Scan(ctx, &triggers)
	if err != nil {
		return nil, fmt.Errorf("peek customer agent triggers: %w", err)
	}
	return triggers, nil
}

// Claim 绑定当前客户输入并在客服门禁内重建最新上下文。
func (f *customerInputFeed) Claim(ctx context.Context, throughSeq int64) (agentruntime.ClaimedInput, error) {
	if throughSeq <= 0 || f.execution.Run.ServiceSessionID == nil {
		return agentruntime.ClaimedInput{}, errors.New("customer agent trigger sequence is invalid")
	}
	var output agentruntime.ClaimedInput
	var previousEndSeq int64
	suppressed := false
	err := f.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		session, err := lockLatestCustomerServiceSession(ctx, tx, f.execution.Run.OrganizationID, f.execution.Run.ConversationID)
		if err != nil {
			return err
		}
		state := &servermodels.ConversationAgentState{}
		if err := tx.NewSelect().Model(state).
			Where("cas.organization_id = ?", f.execution.Run.OrganizationID).
			Where("cas.conversation_id = ?", f.execution.Run.ConversationID).
			Where("cas.agent_identity_id = ?", f.execution.Run.AgentIdentityID).
			For("UPDATE").Scan(ctx); err != nil {
			return fmt.Errorf("lock customer agent input state: %w", err)
		}
		run := &servermodels.AgentRun{}
		if err := tx.NewSelect().Model(run).Where("agr.id = ?", f.execution.Run.ID).For("UPDATE").Scan(ctx); err != nil {
			return fmt.Errorf("lock customer agent run for input claim: %w", err)
		}
		if agentRunStatusTerminal(run.Status) {
			suppressed = true
			return nil
		}
		eligible, err := validateCurrentCustomerRun(ctx, tx, session, run)
		if err != nil {
			return err
		}
		if !eligible {
			if err := suppressCustomerRun(ctx, tx, run, session); err != nil {
				return err
			}
			suppressed = true
			return nil
		}
		if run.Status != string(domain.AgentRunStatusRunning) || state.DesiredSeq <= state.ProcessedSeq {
			return errors.New("customer agent run has no claimable input")
		}
		if run.TriggerEndSeq != nil {
			previousEndSeq = *run.TriggerEndSeq
		}
		claimEnd := min(throughSeq, state.DesiredSeq)
		if claimEnd <= state.ProcessedSeq || claimEnd < run.TriggerStartSeq {
			return errors.New("customer agent input boundary is not claimable")
		}
		var claimedSeqs []int64
		if err := tx.NewRaw(`
			UPDATE conversation_agent_triggers
			SET agent_run_id = ?
			WHERE organization_id = ?
				AND conversation_id = ?
				AND agent_identity_id = ?
				AND trigger_type = ?
				AND service_session_id = ?
				AND trigger_seq > ?
				AND trigger_seq <= ?
			RETURNING trigger_seq
		`, run.ID, run.OrganizationID, run.ConversationID, run.AgentIdentityID,
			domain.AgentTriggerTypeCustomerAuto, session.ID, state.ProcessedSeq, claimEnd,
		).Scan(ctx, &claimedSeqs); err != nil {
			return fmt.Errorf("claim customer agent triggers: %w", err)
		}
		if int64(len(claimedSeqs)) != claimEnd-state.ProcessedSeq {
			return errors.New("customer agent trigger sequence is not contiguous")
		}
		if _, err := tx.NewUpdate().Model(run).
			Set("trigger_start_seq = LEAST(trigger_start_seq, ?)", state.ProcessedSeq+1).
			Set("trigger_end_seq = ?", claimEnd).
			Set("updated_at = now()").
			WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("update customer agent run boundary: %w", err)
		}
		messages, err := loadClaimedCustomerMessages(ctx, tx, run.OrganizationID, run.ConversationID, run.AgentIdentityID, claimEnd)
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
		return agentruntime.ClaimedInput{}, errCustomerRunSuppressed
	}
	slog.Info("网站客户 Agent 输入已认领",
		"agent_run_id", f.execution.Run.ID,
		"conversation_id", f.execution.Run.ConversationID,
		"service_session_id", *f.execution.Run.ServiceSessionID,
		"trigger_start_seq", f.execution.Run.TriggerStartSeq,
		"previous_end_seq", previousEndSeq,
		"trigger_end_seq", output.EndSeq,
		"context_message_count", len(output.Messages),
	)
	return output, nil
}

type customerMessageRow struct {
	Body string `bun:"body"`
	Kind string `bun:"kind"`
}

// loadClaimedCustomerMessages 读取不越过已认领 Trigger 的客户会话上下文。
func loadClaimedCustomerMessages(ctx context.Context, db bun.IDB, organizationID, conversationID, agentIdentityID string, endSeq int64) ([]agentruntime.Message, error) {
	boundary := struct {
		OriginatedAt time.Time `bun:"originated_at"`
		SourceOrder  int64     `bun:"source_order"`
		ID           string    `bun:"id"`
	}{}
	if err := db.NewSelect().
		TableExpr("conversation_agent_triggers AS cat").
		ColumnExpr("msg.originated_at, msg.source_order, msg.id").
		Join("JOIN messages AS msg ON msg.id = cat.trigger_message_id AND msg.organization_id = cat.organization_id AND msg.conversation_id = cat.conversation_id").
		Where("cat.organization_id = ?", organizationID).
		Where("cat.conversation_id = ?", conversationID).
		Where("cat.agent_identity_id = ?", agentIdentityID).
		Where("cat.trigger_type = ?", domain.AgentTriggerTypeCustomerAuto).
		Where("cat.trigger_seq = ?", endSeq).
		Scan(ctx, &boundary); err != nil {
		return nil, fmt.Errorf("load customer agent input boundary: %w", err)
	}
	rows := make([]customerMessageRow, 0, agentHistoryLimit)
	if err := db.NewSelect().
		TableExpr("messages AS msg").
		ColumnExpr("msg.body, cs.kind").
		Join("JOIN conversation_participants AS cp ON cp.id = msg.sender_participant_id AND cp.organization_id = msg.organization_id AND cp.conversation_id = msg.conversation_id").
		Join("JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id").
		Where("msg.organization_id = ?", organizationID).
		Where("msg.conversation_id = ?", conversationID).
		Where("msg.type = ?", domain.MessageTypeText).
		Where("msg.deleted_at IS NULL").
		Where("cs.kind IN (?, ?)", domain.ChatSubjectKindContact, domain.ChatSubjectKindOrganizationIdentity).
		Where("(msg.originated_at, msg.source_order, msg.id) <= (?, ?, ?)", boundary.OriginatedAt, boundary.SourceOrder, boundary.ID).
		OrderExpr("msg.originated_at DESC, msg.source_order DESC, msg.id DESC").
		Limit(agentHistoryLimit).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("load claimed customer conversation context: %w", err)
	}
	slices.Reverse(rows)
	messages := make([]agentruntime.Message, 0, len(rows))
	for _, row := range rows {
		role := agentruntime.MessageRoleAssistant
		if domain.ChatSubjectKind(row.Kind) == domain.ChatSubjectKindContact {
			role = agentruntime.MessageRoleUser
		}
		messages = append(messages, agentruntime.Message{Role: role, Content: row.Body})
	}
	return messages, nil
}

// completeCustomer 写入客户 Agent 最终回复并按实际消费边界补后续运行。
func (a *ExecuteAction) completeCustomer(ctx context.Context, execution executionContext, result agentruntime.RunResult) error {
	content := strings.TrimSpace(result.Content)
	if content == "" || result.EndSeq <= 0 {
		return errors.New("customer agent runtime returned an invalid result")
	}
	usage, err := json.Marshal(result.Usage)
	if err != nil {
		return fmt.Errorf("encode customer agent run usage: %w", err)
	}
	messageID := uuid.NewV7().String()
	suppressed := false
	completed := false
	err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		session, err := lockLatestCustomerServiceSession(ctx, tx, execution.Run.OrganizationID, execution.Run.ConversationID)
		if err != nil {
			return err
		}
		state := &servermodels.ConversationAgentState{}
		if err := tx.NewSelect().Model(state).
			Where("cas.organization_id = ?", execution.Run.OrganizationID).
			Where("cas.conversation_id = ?", execution.Run.ConversationID).
			Where("cas.agent_identity_id = ?", execution.Run.AgentIdentityID).
			For("UPDATE").Scan(ctx); err != nil {
			return fmt.Errorf("lock customer agent state for completion: %w", err)
		}
		run := &servermodels.AgentRun{}
		if err := tx.NewSelect().Model(run).Where("agr.id = ?", execution.Run.ID).For("UPDATE").Scan(ctx); err != nil {
			return fmt.Errorf("lock customer agent run for completion: %w", err)
		}
		if agentRunStatusTerminal(run.Status) {
			return nil
		}
		eligible, err := validateCurrentCustomerRun(ctx, tx, session, run)
		if err != nil {
			return err
		}
		if !eligible {
			if err := suppressCustomerRun(ctx, tx, run, session); err != nil {
				return err
			}
			suppressed = true
			return nil
		}
		if run.Status != string(domain.AgentRunStatusRunning) || run.TriggerEndSeq == nil || *run.TriggerEndSeq != result.EndSeq || run.TriggerStartSeq != state.ProcessedSeq+1 {
			return errors.New("customer agent completion boundary is inconsistent")
		}
		participantID, err := ensureCustomerAgentParticipant(ctx, tx, run.OrganizationID, run.ConversationID, run.AgentIdentityID)
		if err != nil {
			return err
		}
		idempotencyKey := "agent:" + run.ID
		originatedAt := time.Now().UTC()
		message := &servermodels.Message{
			ID: messageID, OrganizationID: run.OrganizationID, ConversationID: run.ConversationID,
			ServiceSessionID: &session.ID, SenderParticipantID: &participantID,
			Type: string(domain.MessageTypeText), Body: content, IdempotencyKey: &idempotencyKey,
			OriginatedAt: originatedAt,
		}
		if _, err := tx.NewInsert().Model(message).
			Column("id", "organization_id", "conversation_id", "service_session_id", "sender_participant_id", "type", "body", "idempotency_key", "originated_at").
			Returning("*").Exec(ctx); err != nil {
			return fmt.Errorf("create customer agent response message: %w", err)
		}
		if err := updateCustomerAgentSummaries(ctx, tx, session, message); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model(run).
			Set("status = ?", domain.AgentRunStatusSucceeded).
			Set("response_message_id = ?", message.ID).
			Set("usage = ?::jsonb", string(usage)).
			Set("last_error = NULL").
			Set("error_code = NULL").
			Set("completed_at = now()").
			Set("updated_at = now()").
			WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("complete customer agent run: %w", err)
		}
		if _, err := tx.NewUpdate().Model(state).
			Set("processed_seq = ?", result.EndSeq).
			Set("updated_at = now()").
			WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("advance customer agent processed sequence: %w", err)
		}
		if state.DesiredSeq > result.EndSeq {
			if err := a.enqueueNextCustomerRun(ctx, tx, session, run, result.EndSeq+1); err != nil {
				return err
			}
		}
		completed = true
		return nil
	})
	if err != nil {
		return err
	}
	if suppressed {
		slog.Warn("网站客户 Agent 迟到结果已抑制",
			"agent_run_id", execution.Run.ID,
			"conversation_id", execution.Run.ConversationID,
		)
	}
	if completed {
		slog.Info("网站客户 Agent 运行完成",
			"agent_run_id", execution.Run.ID,
			"conversation_id", execution.Run.ConversationID,
			"service_session_id", *execution.Run.ServiceSessionID,
			"trigger_start_seq", execution.Run.TriggerStartSeq,
			"trigger_end_seq", result.EndSeq,
			"response_message_id", messageID,
		)
	}
	return nil
}

// failCustomer 终结客户 Agent 失败批次并按资格补后续输入。
func (a *ExecuteAction) failCustomer(ctx context.Context, initial *servermodels.AgentRun, message string) (bool, error) {
	terminal := false
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		session, err := lockLatestCustomerServiceSession(ctx, tx, initial.OrganizationID, initial.ConversationID)
		if err != nil {
			return err
		}
		state := &servermodels.ConversationAgentState{}
		if err := tx.NewSelect().Model(state).
			Where("cas.organization_id = ?", initial.OrganizationID).
			Where("cas.conversation_id = ?", initial.ConversationID).
			Where("cas.agent_identity_id = ?", initial.AgentIdentityID).
			For("UPDATE").Scan(ctx); err != nil {
			return err
		}
		run := &servermodels.AgentRun{}
		if err := tx.NewSelect().Model(run).Where("agr.id = ?", initial.ID).For("UPDATE").Scan(ctx); err != nil {
			return err
		}
		if agentRunStatusTerminal(run.Status) {
			terminal = true
			return nil
		}
		eligible, err := validateCurrentCustomerRun(ctx, tx, session, run)
		if err != nil {
			return err
		}
		if !eligible {
			if err := suppressCustomerRun(ctx, tx, run, session); err != nil {
				return err
			}
			terminal = true
			return nil
		}
		if run.Status != string(domain.AgentRunStatusQueued) && run.Status != string(domain.AgentRunStatusRunning) {
			return fmt.Errorf("cannot fail customer agent run in status %q", run.Status)
		}
		failureEnd := run.TriggerStartSeq
		if run.TriggerEndSeq != nil {
			failureEnd = *run.TriggerEndSeq
		}
		if run.TriggerStartSeq != state.ProcessedSeq+1 || failureEnd < run.TriggerStartSeq || failureEnd > state.DesiredSeq {
			return errors.New("customer agent failure boundary is inconsistent")
		}
		var failedSeqs []int64
		if err := tx.NewRaw(`
			UPDATE conversation_agent_triggers
			SET agent_run_id = ?
			WHERE organization_id = ?
				AND conversation_id = ?
				AND agent_identity_id = ?
				AND trigger_type = ?
				AND service_session_id = ?
				AND trigger_seq > ?
				AND trigger_seq <= ?
			RETURNING trigger_seq
		`, run.ID, run.OrganizationID, run.ConversationID, run.AgentIdentityID,
			domain.AgentTriggerTypeCustomerAuto, session.ID, state.ProcessedSeq, failureEnd,
		).Scan(ctx, &failedSeqs); err != nil {
			return err
		}
		if int64(len(failedSeqs)) != failureEnd-state.ProcessedSeq {
			return errors.New("failed customer agent trigger sequence is not contiguous")
		}
		if _, err := tx.NewUpdate().Model(run).
			Set("status = ?", domain.AgentRunStatusFailed).
			Set("trigger_end_seq = ?", failureEnd).
			Set("last_error = ?", message).
			Set("completed_at = now()").
			Set("updated_at = now()").
			WherePK().Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model(state).
			Set("processed_seq = ?", failureEnd).
			Set("updated_at = now()").
			WherePK().Exec(ctx); err != nil {
			return err
		}
		if state.DesiredSeq > failureEnd {
			if err := a.enqueueNextCustomerRun(ctx, tx, session, run, failureEnd+1); err != nil {
				return err
			}
		}
		return nil
	})
	return terminal, err
}

// validateCurrentCustomerRun 校验运行仍属于当前开放周期和有效 AI 客服。
func validateCurrentCustomerRun(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession, run *servermodels.AgentRun) (bool, error) {
	if run.ServiceSessionID == nil || *run.ServiceSessionID != session.ID ||
		domain.ServiceSessionStatus(session.Status) != domain.ServiceSessionStatusOpen ||
		session.AssigneeIdentityID == nil || *session.AssigneeIdentityID != run.AgentIdentityID {
		return false, nil
	}
	_, eligible, err := loadCustomerAgentEligibility(ctx, db, session, run.AgentRevisionID)
	return eligible, err
}

// suppressCustomerRun 把资格变化后的客户运行收敛为取消终态。
func suppressCustomerRun(ctx context.Context, db bun.IDB, run *servermodels.AgentRun, session *servermodels.ServiceSession) error {
	var errorCode any
	lastError := "customer agent eligibility changed"
	switch {
	case domain.ServiceSessionStatus(session.Status) != domain.ServiceSessionStatusOpen:
		errorCode = domain.AgentRunErrorCodeSessionClosed
		lastError = "customer service session closed"
	case run.ServiceSessionID == nil || *run.ServiceSessionID != session.ID || session.AssigneeIdentityID == nil || *session.AssigneeIdentityID != run.AgentIdentityID:
		errorCode = domain.AgentRunErrorCodeAssigneeChanged
		lastError = "customer service assignee changed"
	default:
		errorCode = nil
	}
	_, err := db.NewUpdate().Model(run).
		Set("status = ?", domain.AgentRunStatusCancelled).
		Set("error_code = ?", errorCode).
		Set("last_error = ?", lastError).
		Set("completed_at = now()").
		Set("updated_at = now()").
		WherePK().
		Where("status IN (?, ?)", domain.AgentRunStatusQueued, domain.AgentRunStatusRunning).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("suppress customer agent run: %w", err)
	}
	return nil
}

// enqueueNextCustomerRun 在当前负责人仍合格时投递未消费输入。
func (a *ExecuteAction) enqueueNextCustomerRun(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession, run *servermodels.AgentRun, startSeq int64) error {
	eligibility, eligible, err := loadCustomerAgentEligibility(ctx, db, session, "")
	if err != nil {
		return err
	}
	if !eligible {
		return nil
	}
	_, err = insertAndEnqueueCustomerRun(
		ctx, db, a.enqueuer, run.OrganizationID, run.ConversationID, run.AgentIdentityID,
		eligibility.RevisionID, session.ID, startSeq,
	)
	return err
}

// ensureCustomerAgentParticipant 取得或创建客户会话中的 Agent 参与者。
func ensureCustomerAgentParticipant(ctx context.Context, db bun.IDB, organizationID, conversationID, agentIdentityID string) (string, error) {
	subject := &servermodels.ChatSubject{}
	err := db.NewSelect().Model(subject).
		Where("cs.organization_id = ?", organizationID).
		Where("cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Where("cs.source_id = ?", agentIdentityID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		subject = &servermodels.ChatSubject{
			ID: uuid.NewV7().String(), OrganizationID: organizationID,
			Kind: string(domain.ChatSubjectKindOrganizationIdentity), SourceID: agentIdentityID,
		}
		if _, err := db.NewInsert().Model(subject).
			Column("id", "organization_id", "kind", "source_id").Exec(ctx); err != nil {
			return "", fmt.Errorf("create customer agent chat subject: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("load customer agent chat subject: %w", err)
	}
	participant := &servermodels.ConversationParticipant{}
	err = db.NewSelect().Model(participant).
		Where("cp.organization_id = ?", organizationID).
		Where("cp.conversation_id = ?", conversationID).
		Where("cp.subject_id = ?", subject.ID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		participant = &servermodels.ConversationParticipant{
			ID: uuid.NewV7().String(), OrganizationID: organizationID, ConversationID: conversationID,
			SubjectID: subject.ID, Role: string(domain.ConversationParticipantRoleMember),
		}
		if _, err := db.NewInsert().Model(participant).
			Column("id", "organization_id", "conversation_id", "subject_id", "role").Exec(ctx); err != nil {
			return "", fmt.Errorf("create customer agent conversation participant: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("load customer agent conversation participant: %w", err)
	} else if participant.LeftAt != nil {
		if _, err := db.NewUpdate().Model(participant).
			Set("left_at = NULL").
			Set("role = ?", domain.ConversationParticipantRoleMember).
			Set("updated_at = now()").
			WherePK().Exec(ctx); err != nil {
			return "", fmt.Errorf("restore customer agent conversation participant: %w", err)
		}
	}
	return participant.ID, nil
}

// updateCustomerAgentSummaries 更新客服首响和两层消息摘要。
func updateCustomerAgentSummaries(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession, message *servermodels.Message) error {
	if _, err := db.NewUpdate().Model(session).
		Set("first_response_at = COALESCE(first_response_at, ?)", message.OriginatedAt).
		Set("last_message_id = ?", message.ID).
		Set("last_message_at = ?", message.OriginatedAt).
		Set("last_message_source_order = ?", message.SourceOrder).
		Set("updated_at = now()").
		WherePK().
		Where("organization_id = ?", message.OrganizationID).
		Where("status = ?", domain.ServiceSessionStatusOpen).
		Where("(last_message_at, last_message_source_order, last_message_id) < (?, ?, ?)", message.OriginatedAt, message.SourceOrder, message.ID).
		Exec(ctx); err != nil {
		return fmt.Errorf("update service session after customer agent response: %w", err)
	}
	if _, err := db.NewUpdate().Model((*servermodels.Conversation)(nil)).
		Set("last_message_id = ?", message.ID).
		Set("last_message_at = ?", message.OriginatedAt).
		Set("last_message_source_order = ?", message.SourceOrder).
		Set("updated_at = now()").
		Where("id = ?", message.ConversationID).
		Where("organization_id = ?", message.OrganizationID).
		Where("last_message_at IS NULL OR (last_message_at, last_message_source_order, last_message_id) < (?, ?, ?)", message.OriginatedAt, message.SourceOrder, message.ID).
		Exec(ctx); err != nil {
		return fmt.Errorf("update conversation after customer agent response: %w", err)
	}
	return nil
}
