//go:build server

package agentrun

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

type groupRunPolicy struct {
	enqueuer servertask.TxEnqueuer
}

type groupAgentEligibility struct {
	ParticipantID string `bun:"participant_id"`
	RevisionID    string `bun:"active_revision_id"`
	Status        string `bun:"status"`
}

// lockContext 先锁群聊再锁 Agent，串行化成员变化、消息写入和账号停用。
func (p groupRunPolicy) lockContext(ctx context.Context, db bun.IDB, run *servermodels.AgentRun) (agentRunPolicyContext, error) {
	group := &servermodels.Conversation{}
	if err := db.NewSelect().Model(group).
		Where("cv.organization_id = ? AND cv.id = ? AND cv.type = ?", run.OrganizationID, run.ConversationID, domain.ConversationTypeGroup).
		For("UPDATE").Scan(ctx); err != nil {
		return agentRunPolicyContext{}, fmt.Errorf("lock agent group: %w", err)
	}
	var agent groupAgentEligibility
	err := db.NewSelect().TableExpr("agents AS a").
		ColumnExpr("a.active_revision_id, a.status, cp.id AS participant_id").
		Join("JOIN chat_subjects AS cs ON cs.organization_id = a.organization_id AND cs.kind = ? AND cs.source_id = a.identity_id", domain.ChatSubjectKindOrganizationIdentity).
		Join("JOIN conversation_participants AS cp ON cp.organization_id = cs.organization_id AND cp.subject_id = cs.id AND cp.conversation_id = ? AND cp.left_at IS NULL", run.ConversationID).
		Where("a.organization_id = ? AND a.identity_id = ?", run.OrganizationID, run.AgentIdentityID).
		For("SHARE OF a").Scan(ctx, &agent)
	if errors.Is(err, sql.ErrNoRows) {
		return agentRunPolicyContext{Group: group}, nil
	}
	if err != nil {
		return agentRunPolicyContext{}, fmt.Errorf("load group agent eligibility: %w", err)
	}
	return agentRunPolicyContext{Group: group, GroupAgent: &agent}, nil
}

// prepareLocked 校验群聊和成员资格，并推进失效运行的消费水位。
func (p groupRunPolicy) prepareLocked(ctx context.Context, db bun.IDB, scope agentRunPolicyContext, run *servermodels.AgentRun) (bool, error) {
	var reason domain.AgentRunErrorCode
	switch {
	case scope.Group.Status != string(domain.ConversationStatusActive):
		reason = domain.AgentRunErrorCodeGroupArchived
	case scope.GroupAgent == nil:
		reason = domain.AgentRunErrorCodeGroupMemberRemoved
	case scope.GroupAgent.Status != string(domain.UserStatusActive):
		reason = domain.AgentRunErrorCodeAgentInactive
	default:
		return true, nil
	}
	return false, CancelGroupRuns(ctx, db, run.OrganizationID, run.ConversationID, run.AgentIdentityID, reason)
}

// loadMessages 按群消息序号读取包含发言者和提醒对象的上下文。
func (p groupRunPolicy) loadMessages(ctx context.Context, db bun.IDB, run *servermodels.AgentRun, endSeq int64) ([]agentruntime.Message, error) {
	return loadClaimedConversationMessages(ctx, db, run, endSeq)
}

// persistResponse 分配群消息序号并原子写入 Agent 回复和会话摘要。
func (p groupRunPolicy) persistResponse(ctx context.Context, db bun.IDB, scope agentRunPolicyContext, run *servermodels.AgentRun, messageID, content string) error {
	var sequence int64
	if err := db.NewUpdate().Model(scope.Group).
		Set("last_group_message_sequence = last_group_message_sequence + 1").
		WherePK().Returning("last_group_message_sequence").Scan(ctx, &sequence); err != nil {
		return fmt.Errorf("allocate agent group message sequence: %w", err)
	}
	key := "agent:" + run.ID
	message := &servermodels.Message{
		ID: messageID, OrganizationID: run.OrganizationID, ConversationID: run.ConversationID,
		SenderParticipantID: &scope.GroupAgent.ParticipantID, GroupMessageSequence: &sequence,
		Type: string(domain.MessageTypeText), Body: content, IdempotencyKey: &key, OriginatedAt: time.Now().UTC(),
	}
	if _, err := db.NewInsert().Model(message).
		Column("id", "organization_id", "conversation_id", "sender_participant_id", "group_message_sequence", "type", "body", "idempotency_key", "originated_at").
		Returning("*").Exec(ctx); err != nil {
		return fmt.Errorf("create group agent reply: %w", err)
	}
	// 群消息按事务内分配的序号推进摘要，不比较消息时间。
	_, err := db.NewUpdate().Model(scope.Group).
		Set("last_message_id = ?, last_message_at = ?, last_message_source_order = ?, updated_at = now()", message.ID, message.OriginatedAt, message.SourceOrder).
		WherePK().Exec(ctx)
	return err
}

// enqueueNext 使用当前版本为同一群成员的剩余提醒投递下一次运行。
func (p groupRunPolicy) enqueueNext(ctx context.Context, db bun.IDB, scope agentRunPolicyContext, run *servermodels.AgentRun, startSeq int64) error {
	_, err := insertAndEnqueueRun(ctx, db, p.enqueuer, agentRunSpec{
		OrganizationID: run.OrganizationID, ConversationID: run.ConversationID,
		AgentIdentityID: run.AgentIdentityID, RevisionID: scope.GroupAgent.RevisionID,
		TriggerType: domain.AgentTriggerTypeMention,
	}, startSeq)
	return err
}
