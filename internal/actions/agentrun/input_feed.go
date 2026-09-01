//go:build server

package agentrun

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

type databaseInputFeed struct {
	db        *bun.DB
	execution executionContext
}

// Peek 返回尚未进入 TurnLoop 缓冲区的连续输入信号。
func (f *databaseInputFeed) Peek(ctx context.Context, afterSeq int64) ([]agentruntime.Trigger, error) {
	triggers := make([]agentruntime.Trigger, 0)
	err := f.db.NewSelect().TableExpr("conversation_agent_triggers AS cat").
		ColumnExpr("cat.trigger_seq AS seq").
		Join("JOIN conversation_agent_states AS cas ON cas.conversation_id = cat.conversation_id AND cas.organization_id = cat.organization_id AND cas.agent_identity_id = cat.agent_identity_id").
		Where("cat.organization_id = ?", f.execution.Run.OrganizationID).
		Where("cat.conversation_id = ?", f.execution.Run.ConversationID).
		Where("cat.agent_identity_id = ?", f.execution.Run.AgentIdentityID).
		Where("cat.trigger_seq > cas.processed_seq").
		Where("cat.trigger_seq > ?", afterSeq).
		OrderExpr("cat.trigger_seq ASC").
		Scan(ctx, &triggers)
	if err != nil {
		return nil, fmt.Errorf("peek agent triggers: %w", err)
	}
	return triggers, nil
}

// Claim 绑定当前所有已持久化输入，并重建截至该边界的会话上下文。
func (f *databaseInputFeed) Claim(ctx context.Context, throughSeq int64) (agentruntime.ClaimedInput, error) {
	if throughSeq <= 0 {
		return agentruntime.ClaimedInput{}, errors.New("agent trigger sequence is invalid")
	}
	var result agentruntime.ClaimedInput
	err := f.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		state := &servermodels.ConversationAgentState{}
		if err := tx.NewSelect().Model(state).
			Where("cas.conversation_id = ?", f.execution.Run.ConversationID).
			Where("cas.organization_id = ?", f.execution.Run.OrganizationID).
			Where("cas.agent_identity_id = ?", f.execution.Run.AgentIdentityID).
			For("UPDATE").Scan(ctx); err != nil {
			return fmt.Errorf("lock agent input state: %w", err)
		}
		run := &servermodels.AgentRun{}
		if err := tx.NewSelect().Model(run).Where("agr.id = ?", f.execution.Run.ID).For("UPDATE").Scan(ctx); err != nil {
			return fmt.Errorf("lock agent run for input claim: %w", err)
		}
		if run.Status != string(domain.AgentRunStatusRunning) || state.DesiredSeq <= state.ProcessedSeq {
			return errors.New("agent run has no claimable input")
		}
		claimEnd := min(throughSeq, state.DesiredSeq)
		if claimEnd <= state.ProcessedSeq || claimEnd < run.TriggerStartSeq {
			return errors.New("agent run input boundary is not claimable")
		}
		var claimedSeqs []int64
		if err := tx.NewRaw(`
			UPDATE conversation_agent_triggers
			SET agent_run_id = ?
			WHERE organization_id = ?
				AND conversation_id = ?
				AND agent_identity_id = ?
				AND trigger_seq > ?
				AND trigger_seq <= ?
			RETURNING trigger_seq
		`, run.ID, run.OrganizationID, run.ConversationID, run.AgentIdentityID, state.ProcessedSeq, claimEnd).Scan(ctx, &claimedSeqs); err != nil {
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
		messages, err := loadClaimedConversationMessages(ctx, tx, run.OrganizationID, run.ConversationID, f.execution.Run.AgentIdentityID, claimEnd)
		if err != nil {
			return err
		}
		result = agentruntime.ClaimedInput{Messages: messages, EndSeq: claimEnd}
		return nil
	})
	return result, err
}

type claimedMessageRow struct {
	Body           string `bun:"body"`
	SenderSourceID string `bun:"sender_source_id"`
}

// loadClaimedConversationMessages 读取不越过已认领 Trigger 的最近会话上下文。
func loadClaimedConversationMessages(ctx context.Context, db bun.IDB, organizationID, conversationID, agentIdentityID string, endSeq int64) ([]agentruntime.Message, error) {
	type boundaryRow struct {
		CreatedAt time.Time `bun:"created_at"`
		ID        string    `bun:"id"`
	}
	boundary := boundaryRow{}
	if err := db.NewSelect().TableExpr("conversation_agent_triggers AS cat").
		ColumnExpr("msg.created_at, msg.id").
		Join("JOIN messages AS msg ON msg.id = cat.trigger_message_id AND msg.organization_id = cat.organization_id AND msg.conversation_id = cat.conversation_id").
		Where("cat.organization_id = ?", organizationID).
		Where("cat.conversation_id = ?", conversationID).
		Where("cat.agent_identity_id = ?", agentIdentityID).
		Where("cat.trigger_seq = ?", endSeq).
		Scan(ctx, &boundary); err != nil {
		return nil, fmt.Errorf("load claimed input boundary: %w", err)
	}
	rows := make([]claimedMessageRow, 0, agentHistoryLimit)
	if err := db.NewSelect().TableExpr("messages AS msg").
		ColumnExpr("msg.body").
		ColumnExpr("cs.source_id AS sender_source_id").
		Join("JOIN conversation_participants AS cp ON cp.id = msg.sender_participant_id AND cp.organization_id = msg.organization_id AND cp.conversation_id = msg.conversation_id").
		Join("JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Where("msg.organization_id = ?", organizationID).
		Where("msg.conversation_id = ?", conversationID).
		Where("msg.type = ?", domain.MessageTypeText).
		Where("msg.deleted_at IS NULL").
		Where("(msg.created_at, msg.id) <= (?, ?)", boundary.CreatedAt, boundary.ID).
		OrderExpr("msg.created_at DESC, msg.id DESC").
		Limit(agentHistoryLimit).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("load claimed conversation context: %w", err)
	}
	slices.Reverse(rows)
	messages := make([]agentruntime.Message, 0, len(rows))
	for _, row := range rows {
		role := agentruntime.MessageRoleUser
		if row.SenderSourceID == agentIdentityID {
			role = agentruntime.MessageRoleAssistant
		}
		messages = append(messages, agentruntime.Message{Role: role, Content: row.Body})
	}
	return messages, nil
}
