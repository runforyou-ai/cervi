//go:build server

// Package knowledgegap 维护待补知识：按转人工、访客评价与周期关闭事件登记 AI 客服缺少知识或可能答错的客服处理周期，由管理员把 AI 起草的问答加入知识库或忽略。
package knowledgegap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// DraftActionName 按周期沟通记录为待补知识起草问答。
const DraftActionName = "knowledge_gap.draft"

// draftMaxAttempts 是起草任务的最大尝试次数。
const draftMaxAttempts = 3

var (
	// ErrNotFound 表示当前企业中不存在指定待补知识。
	ErrNotFound = errors.New("knowledge gap not found")
	// ErrHandled 表示待补知识已加入知识库，不能再次处理。
	ErrHandled = errors.New("knowledge gap already handled")
	// ErrPageSizeInvalid 表示每页数量超出上限。
	ErrPageSizeInvalid = errors.New("knowledge gap page size invalid")
	// ErrStatusInvalid 表示筛选的处理状态不存在。
	ErrStatusInvalid = errors.New("knowledge gap status invalid")
)

// DraftInput 定义一次问答起草任务；RequestedAt 与条目当前的起草请求时间不一致时任务不再生效。
type DraftInput struct {
	OrganizationID string    `json:"organizationId"`
	KnowledgeGapID string    `json:"knowledgeGapId"`
	RequestedAt    time.Time `json:"requestedAt"`
}

// gapHandoffSources 是计入待补知识的转人工原因。
var gapHandoffSources = []domain.AgentHandoffReason{domain.AgentHandoffReasonKnowledgeGap, domain.AgentHandoffReasonInsufficientEvidence}

// RecordClosed 在调用方持有会话锁的事务中处理刚关闭的周期：周期已有待处理条目时按新的沟通重新起草，否则按最近一次知识不足或缺少依据的转人工登记。
func RecordClosed(ctx context.Context, db bun.IDB, enqueuer servertask.TxEnqueuer, session *servermodels.ServiceSession) error {
	pending := make([]servermodels.KnowledgeGap, 0, 1)
	if err := db.NewSelect().Model(&pending).
		Where("kg.organization_id = ? AND kg.service_session_id = ? AND kg.status = ?", session.OrganizationID, session.ID, domain.KnowledgeGapStatusPending).
		For("UPDATE").
		Scan(ctx); err != nil {
		return fmt.Errorf("load pending knowledge gap: %w", err)
	}
	if len(pending) > 0 {
		return redraft(ctx, db, enqueuer, &pending[0])
	}
	var event struct {
		ID             string    `bun:"id"`
		MessageSeq     int64     `bun:"message_seq"`
		CreatedAt      time.Time `bun:"created_at"`
		Reason         string    `bun:"reason"`
		FromIdentityID *string   `bun:"from_identity_id"`
	}
	err := db.NewSelect().
		TableExpr("messages AS m").
		ColumnExpr("m.id, m.message_seq, m.created_at, m.system_event_payload->>'reason' AS reason").
		ColumnExpr("nullif(m.system_event_payload->>'fromIdentityId', '') AS from_identity_id").
		Where("m.organization_id = ? AND m.service_session_id = ?", session.OrganizationID, session.ID).
		Where("m.system_event_type = ?", domain.ConversationSystemEventServiceSessionHandedOff).
		Where("m.system_event_payload->>'reason' IN (?)", bun.In(gapHandoffSources)).
		OrderExpr("m.message_seq DESC").
		Limit(1).
		Scan(ctx, &event)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load knowledge gap handoff: %w", err)
	}
	questionID, err := customerQuestion(ctx, db, session, &event.MessageSeq)
	if err != nil {
		return err
	}
	return record(ctx, db, enqueuer, &servermodels.KnowledgeGap{
		OrganizationID: session.OrganizationID, ServiceSessionID: session.ID, ConversationID: session.ConversationID,
		Source: event.Reason, TriggerMessageID: event.ID, OccurredAt: event.CreatedAt, QuestionMessageID: questionID, AgentIdentityID: event.FromIdentityID,
	})
}

// RecordAIReview 在调用方事务中为 AI 员工关闭的周期登记需要复核的待补知识，triggerMessageID 为访客评价或周期关闭事件；周期不是 AI 员工关闭或已有待处理条目时不登记。
func RecordAIReview(ctx context.Context, db bun.IDB, enqueuer servertask.TxEnqueuer, session *servermodels.ServiceSession,
	source domain.KnowledgeGapSource, triggerMessageID string, occurredAt time.Time) error {
	if session.ClosedByIdentityID == nil {
		return nil
	}
	closedByAgent, err := db.NewSelect().Model((*servermodels.OrganizationIdentity)(nil)).
		Where("oi.organization_id = ? AND oi.id = ? AND oi.type = ?", session.OrganizationID, *session.ClosedByIdentityID, domain.OrganizationIdentityTypeAgent).
		Exists(ctx)
	if err != nil {
		return fmt.Errorf("load service session closer: %w", err)
	}
	if !closedByAgent {
		return nil
	}
	questionID, err := customerQuestion(ctx, db, session, nil)
	if err != nil {
		return err
	}
	return record(ctx, db, enqueuer, &servermodels.KnowledgeGap{
		OrganizationID: session.OrganizationID, ServiceSessionID: session.ID, ConversationID: session.ConversationID,
		Source: string(source), TriggerMessageID: triggerMessageID, OccurredAt: occurredAt, QuestionMessageID: questionID, AgentIdentityID: session.ClosedByIdentityID,
	})
}

// record 登记待补知识并投递起草任务；触发事件已登记或周期已有待处理条目时保持不变。
func record(ctx context.Context, db bun.IDB, enqueuer servertask.TxEnqueuer, gap *servermodels.KnowledgeGap) error {
	inserted := make([]servermodels.KnowledgeGap, 0, 1)
	if _, err := db.NewInsert().Model(gap).
		Column("organization_id", "service_session_id", "conversation_id", "source", "trigger_message_id", "occurred_at", "question_message_id", "agent_identity_id").
		On("CONFLICT DO NOTHING").
		Returning("id, draft_requested_at").
		Exec(ctx, &inserted); err != nil {
		return fmt.Errorf("record knowledge gap: %w", err)
	}
	if len(inserted) == 0 {
		return nil
	}
	return enqueueDraft(ctx, db, enqueuer, gap.OrganizationID, &inserted[0])
}

// redraft 清除待处理条目的草稿并重新请求起草，旧的起草任务不再写入。
func redraft(ctx context.Context, db bun.IDB, enqueuer servertask.TxEnqueuer, gap *servermodels.KnowledgeGap) error {
	if _, err := db.NewUpdate().Model(gap).
		Set("draft_status = ?", domain.KnowledgeGapDraftStatusPending).
		Set("draft_requested_at = clock_timestamp()").
		Set("draft_question = NULL").
		Set("draft_similar_questions = NULL").
		Set("draft_answer = NULL").
		Set("updated_at = now()").
		WherePK().
		Returning("draft_requested_at").
		Exec(ctx); err != nil {
		return fmt.Errorf("reset knowledge gap draft: %w", err)
	}
	return enqueueDraft(ctx, db, enqueuer, gap.OrganizationID, gap)
}

// enqueueDraft 在调用方事务中投递起草任务。
func enqueueDraft(ctx context.Context, db bun.IDB, enqueuer servertask.TxEnqueuer, organizationID string, gap *servermodels.KnowledgeGap) error {
	if _, err := enqueuer.EnqueueIn(ctx, db, DraftActionName, DraftInput{OrganizationID: organizationID, KnowledgeGapID: gap.ID, RequestedAt: gap.DraftRequestedAt},
		servertask.EnqueueOptions{MaxAttempts: draftMaxAttempts}); err != nil {
		return fmt.Errorf("enqueue %s: %w", DraftActionName, err)
	}
	return nil
}

// customerQuestion 返回周期内客户的文本提问消息编号：给出序号时取其之前最后一条，否则取周期内第一条；没有时返回 nil。
func customerQuestion(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession, beforeSeq *int64) (*string, error) {
	ids := make([]string, 0, 1)
	query := db.NewSelect().
		TableExpr("messages AS m").
		Column("m.id").
		Join("JOIN conversation_participants AS cp ON cp.id = m.sender_participant_id AND cp.organization_id = m.organization_id").
		Join("JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id").
		Where("m.organization_id = ? AND m.service_session_id = ?", session.OrganizationID, session.ID).
		Where("m.type = ? AND m.deleted_at IS NULL AND cs.kind = ?", domain.MessageTypeText, domain.ChatSubjectKindContact).
		Limit(1)
	if beforeSeq != nil {
		query = query.Where("m.message_seq < ?", *beforeSeq).OrderExpr("m.message_seq DESC")
	} else {
		query = query.OrderExpr("m.message_seq ASC")
	}
	if err := query.Scan(ctx, &ids); err != nil {
		return nil, fmt.Errorf("load knowledge gap question: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return &ids[0], nil
}
