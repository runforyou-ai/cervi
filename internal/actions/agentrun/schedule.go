//go:build server

package agentrun

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// Scheduler 在消息事务内追加 Agent 输入并创建可靠任务。
type Scheduler struct {
	enqueuer servertask.TxEnqueuer
}

type laneSequence struct {
	LaneID       string `bun:"lane_id"`
	DesiredSeq   int64  `bun:"desired_seq"`
	ProcessedSeq int64  `bun:"processed_seq"`
}

type agentRunSpec struct {
	OrganizationID  string
	ConversationID  string
	AgentIdentityID string
	RevisionID      string
	ScopeKind       domain.AgentExecutionScopeKind
	ScopeID         string
	Kind            domain.AgentInputKind
	SourceSubjectID string
	SourceOrdinal   int
}

// NewScheduler 创建 Agent 运行调度器。
func NewScheduler(enqueuer servertask.TxEnqueuer) *Scheduler {
	return &Scheduler{enqueuer: enqueuer}
}

// Schedule 在调用方已锁定会话并保存个人状态的事务内追加 AI 聊天输入。
func (s *Scheduler) Schedule(ctx context.Context, db bun.IDB, organizationID, conversationID, agentIdentityID, revisionID, messageID, senderSubjectID string) error {
	return s.scheduleInput(ctx, db, agentRunSpec{
		OrganizationID: organizationID, ConversationID: conversationID,
		AgentIdentityID: agentIdentityID, RevisionID: revisionID,
		ScopeKind: domain.AgentExecutionScopeConversation, ScopeID: conversationID,
		Kind: domain.AgentInputKindAgentDirect, SourceSubjectID: senderSubjectID,
	}, messageID)
}

// scheduleInput 追加一条持久输入并确保对应执行范围已有在途运行。
func (s *Scheduler) scheduleInput(ctx context.Context, db bun.IDB, spec agentRunSpec, messageID string) error {
	if s == nil || s.enqueuer == nil {
		return errors.New("agent run scheduler is unavailable")
	}
	sequence, err := advanceLaneSequence(ctx, db, spec)
	if err != nil {
		return err
	}
	input := &servermodels.AgentInput{
		ID: uuid.NewV7().String(), OrganizationID: spec.OrganizationID, LaneID: sequence.LaneID,
		InputSeq: sequence.DesiredSeq, Kind: string(spec.Kind),
		SourceMessageID: messageID, SourceSubjectID: spec.SourceSubjectID, SourceOrdinal: spec.SourceOrdinal,
	}
	if _, err := db.NewInsert().Model(input).
		Column("id", "organization_id", "lane_id", "input_seq", "kind", "source_message_id", "source_subject_id", "source_ordinal").
		Exec(ctx); err != nil {
		return fmt.Errorf("create agent input: %w", err)
	}
	active, err := db.NewSelect().Model((*servermodels.AgentRun)(nil)).
		Where("agr.organization_id = ?", spec.OrganizationID).
		Where("agr.scope_kind = ? AND agr.scope_id = ?", spec.ScopeKind, spec.ScopeID).
		Where("agr.status IN (?, ?)", domain.AgentRunStatusQueued, domain.AgentRunStatusRunning).
		Exists(ctx)
	if err != nil {
		return fmt.Errorf("check active agent run: %w", err)
	}
	if active {
		return nil
	}
	_, err = insertAndEnqueueRun(ctx, db, s.enqueuer, spec, sequence.LaneID, sequence.ProcessedSeq+1)
	return err
}

// advanceLaneSequence 建立或锁定执行范围的输入队列并分配下一条输入序号。
func advanceLaneSequence(ctx context.Context, db bun.IDB, spec agentRunSpec) (laneSequence, error) {
	sequence := laneSequence{}
	if err := db.NewRaw(`
		INSERT INTO agent_lanes (
			organization_id, conversation_id, agent_identity_id, scope_kind, scope_id, desired_seq, processed_seq
		)
		VALUES (?, ?, ?, ?, ?, 1, 0)
		ON CONFLICT (organization_id, scope_kind, scope_id, agent_identity_id) DO UPDATE
		SET desired_seq = agent_lanes.desired_seq + 1,
			updated_at = now()
		RETURNING id AS lane_id, desired_seq, processed_seq
	`, spec.OrganizationID, spec.ConversationID, spec.AgentIdentityID, spec.ScopeKind, spec.ScopeID).
		Scan(ctx, &sequence); err != nil {
		return laneSequence{}, fmt.Errorf("advance agent lane input sequence: %w", err)
	}
	return sequence, nil
}

// insertAndEnqueueRun 创建 Agent 业务运行并投递隔离 Worker。
func insertAndEnqueueRun(ctx context.Context, db bun.IDB, enqueuer servertask.TxEnqueuer, spec agentRunSpec, laneID string, startSeq int64) (string, error) {
	run := &servermodels.AgentRun{
		ID: uuid.NewV7().String(), OrganizationID: spec.OrganizationID, ConversationID: spec.ConversationID,
		AgentIdentityID: spec.AgentIdentityID, AgentRevisionID: spec.RevisionID, LaneID: laneID,
		ScopeKind: string(spec.ScopeKind), ScopeID: spec.ScopeID,
		Status: string(domain.AgentRunStatusQueued), InputStartSeq: startSeq,
	}
	if _, err := db.NewInsert().Model(run).
		Column("id", "organization_id", "conversation_id", "agent_identity_id", "agent_revision_id", "lane_id", "scope_kind", "scope_id", "status", "input_start_seq").
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
