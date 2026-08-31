//go:build server

package agentrun

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// Scheduler 在消息事务内创建 Agent Trigger 和可靠任务。
type Scheduler struct {
	enqueuer servertask.TxEnqueuer
}

type conversationSequence struct {
	DesiredSeq   int64 `bun:"desired_seq"`
	ProcessedSeq int64 `bun:"processed_seq"`
}

// NewScheduler 创建 Agent 单聊运行调度器。
func NewScheduler(enqueuer servertask.TxEnqueuer) *Scheduler {
	return &Scheduler{enqueuer: enqueuer}
}

// Schedule 把一条新用户消息追加到 Agent 持久化输入流。
func (s *Scheduler) Schedule(ctx context.Context, db bun.IDB, organizationID, conversationID, agentIdentityID, revisionID, messageID string) error {
	if s == nil || s.enqueuer == nil {
		return errors.New("agent run scheduler is unavailable")
	}
	sequence := conversationSequence{}
	err := db.NewRaw(`
		INSERT INTO conversation_agent_states (
			conversation_id, organization_id, agent_identity_id, desired_seq, processed_seq
		)
		VALUES (?, ?, ?, 1, 0)
		ON CONFLICT (conversation_id, agent_identity_id) DO UPDATE
		SET desired_seq = conversation_agent_states.desired_seq + 1,
			updated_at = now()
		WHERE conversation_agent_states.organization_id = EXCLUDED.organization_id
		RETURNING desired_seq, processed_seq
	`, conversationID, organizationID, agentIdentityID).Scan(ctx, &sequence)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("agent conversation state does not match target agent")
	}
	if err != nil {
		return fmt.Errorf("advance agent conversation input sequence: %w", err)
	}
	triggerID := uuid.NewV7()
	trigger := &servermodels.ConversationAgentTrigger{
		ID: triggerID.String(), OrganizationID: organizationID, ConversationID: conversationID,
		AgentIdentityID: agentIdentityID, TriggerType: string(domain.AgentTriggerTypeDirect),
		TriggerSeq: sequence.DesiredSeq, TriggerMessageID: messageID,
	}
	if _, err := db.NewInsert().Model(trigger).
		Column("id", "organization_id", "conversation_id", "agent_identity_id", "trigger_type", "trigger_seq", "trigger_message_id").
		Exec(ctx); err != nil {
		return fmt.Errorf("create agent trigger: %w", err)
	}

	active, err := db.NewSelect().Model((*servermodels.AgentRun)(nil)).
		Where("agr.organization_id = ?", organizationID).
		Where("agr.conversation_id = ?", conversationID).
		Where("agr.agent_identity_id = ?", agentIdentityID).
		Where("agr.status IN (?, ?)", domain.AgentRunStatusQueued, domain.AgentRunStatusRunning).
		Exists(ctx)
	if err != nil {
		return fmt.Errorf("check active agent run: %w", err)
	}
	if active {
		return nil
	}
	_, err = insertAndEnqueueRun(ctx, db, s.enqueuer, organizationID, conversationID, agentIdentityID, revisionID, sequence.ProcessedSeq+1)
	return err
}

// insertAndEnqueueRun 创建 Agent 业务运行并投递隔离 Worker。
func insertAndEnqueueRun(ctx context.Context, db bun.IDB, enqueuer servertask.TxEnqueuer, organizationID, conversationID, agentIdentityID, revisionID string, startSeq int64) (string, error) {
	runID := uuid.NewV7()
	run := &servermodels.AgentRun{
		ID: runID.String(), OrganizationID: organizationID, ConversationID: conversationID,
		AgentIdentityID: agentIdentityID, AgentRevisionID: revisionID, TriggerType: string(domain.AgentTriggerTypeDirect), Status: string(domain.AgentRunStatusQueued),
		TriggerStartSeq: startSeq,
	}
	if _, err := db.NewInsert().Model(run).
		Column("id", "organization_id", "conversation_id", "agent_identity_id", "agent_revision_id", "trigger_type", "status", "trigger_start_seq").
		Exec(ctx); err != nil {
		return "", fmt.Errorf("create agent run: %w", err)
	}
	if _, err := enqueuer.EnqueueIn(ctx, db, RunActionName, RunInput{RunID: run.ID}, servertask.EnqueueOptions{
		Queue: servertask.QueueAgent, MaxAttempts: 3,
		IdempotencyKey: "agent:" + run.ID,
		TriggerType:    servertask.TriggerBusiness,
	}); err != nil {
		return "", fmt.Errorf("enqueue agent run: %w", err)
	}
	return run.ID, nil
}
