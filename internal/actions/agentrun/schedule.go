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

type agentRunSpec struct {
	OrganizationID   string
	ConversationID   string
	AgentIdentityID  string
	RevisionID       string
	TriggerType      domain.AgentTriggerType
	ServiceSessionID *string
}

// NewScheduler 创建 Agent 运行调度器。
func NewScheduler(enqueuer servertask.TxEnqueuer) *Scheduler {
	return &Scheduler{enqueuer: enqueuer}
}

// Schedule 把一条新用户消息追加到 Agent 持久化输入流。
func (s *Scheduler) Schedule(ctx context.Context, db bun.IDB, organizationID, conversationID, agentIdentityID, revisionID, messageID string) error {
	return s.scheduleInput(ctx, db, agentRunSpec{
		OrganizationID: organizationID, ConversationID: conversationID,
		AgentIdentityID: agentIdentityID, RevisionID: revisionID,
		TriggerType: domain.AgentTriggerTypeDirect,
	}, messageID)
}

// scheduleInput 追加一条持久输入并确保对应的 Agent Run 已投递。
func (s *Scheduler) scheduleInput(ctx context.Context, db bun.IDB, spec agentRunSpec, messageID string) error {
	if s == nil || s.enqueuer == nil {
		return errors.New("agent run scheduler is unavailable")
	}
	if err := validateAgentRunScope(spec.TriggerType, spec.ServiceSessionID); err != nil {
		return err
	}
	sequence, err := advanceAgentSequence(ctx, db, spec.OrganizationID, spec.ConversationID, spec.AgentIdentityID)
	if err != nil {
		return err
	}
	trigger := &servermodels.ConversationAgentTrigger{
		ID: uuid.NewV7().String(), OrganizationID: spec.OrganizationID, ConversationID: spec.ConversationID,
		AgentIdentityID: spec.AgentIdentityID, TriggerType: string(spec.TriggerType),
		ServiceSessionID: spec.ServiceSessionID, TriggerSeq: sequence.DesiredSeq, TriggerMessageID: messageID,
	}
	if _, err := db.NewInsert().Model(trigger).
		Column("id", "organization_id", "conversation_id", "agent_identity_id", "trigger_type", "service_session_id", "trigger_seq", "trigger_message_id").
		Exec(ctx); err != nil {
		return fmt.Errorf("create agent trigger: %w", err)
	}

	active := &servermodels.AgentRun{}
	err = db.NewSelect().Model(active).
		Where("agr.organization_id = ?", spec.OrganizationID).
		Where("agr.conversation_id = ?", spec.ConversationID).
		Where("agr.agent_identity_id = ?", spec.AgentIdentityID).
		Where("agr.status IN (?, ?)", domain.AgentRunStatusQueued, domain.AgentRunStatusRunning).
		Scan(ctx)
	if err == nil {
		// 判断活动运行是否消费同一类输入。
		matches := active.TriggerType == string(spec.TriggerType)
		if active.ServiceSessionID == nil || spec.ServiceSessionID == nil {
			matches = matches && active.ServiceSessionID == nil && spec.ServiceSessionID == nil
		} else {
			matches = matches && *active.ServiceSessionID == *spec.ServiceSessionID
		}
		if !matches {
			return errors.New("active agent run does not match scheduled input")
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check active agent run: %w", err)
	}
	_, err = insertAndEnqueueRun(ctx, db, s.enqueuer, spec, sequence.ProcessedSeq+1)
	return err
}

// advanceAgentSequence 分配会话 Agent 的下一条连续触发序号。
func advanceAgentSequence(ctx context.Context, db bun.IDB, organizationID, conversationID, agentIdentityID string) (conversationSequence, error) {
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
		return conversationSequence{}, errors.New("agent conversation state does not match target agent")
	}
	if err != nil {
		return conversationSequence{}, fmt.Errorf("advance agent conversation input sequence: %w", err)
	}
	return sequence, nil
}

// insertAndEnqueueRun 创建 Agent 业务运行并投递隔离 Worker。
func insertAndEnqueueRun(ctx context.Context, db bun.IDB, enqueuer servertask.TxEnqueuer, spec agentRunSpec, startSeq int64) (string, error) {
	if err := validateAgentRunScope(spec.TriggerType, spec.ServiceSessionID); err != nil {
		return "", err
	}
	run := &servermodels.AgentRun{
		ID: uuid.NewV7().String(), OrganizationID: spec.OrganizationID, ConversationID: spec.ConversationID,
		AgentIdentityID: spec.AgentIdentityID, AgentRevisionID: spec.RevisionID,
		TriggerType: string(spec.TriggerType), ServiceSessionID: spec.ServiceSessionID,
		Status: string(domain.AgentRunStatusQueued), TriggerStartSeq: startSeq,
	}
	if _, err := db.NewInsert().Model(run).
		Column("id", "organization_id", "conversation_id", "agent_identity_id", "agent_revision_id", "trigger_type", "service_session_id", "status", "trigger_start_seq").
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
