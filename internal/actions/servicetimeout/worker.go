//go:build server

// Package servicetimeout 按企业超时设置提醒负责人和队列客服处理等待中的客户，并回收负责人超时未回复的客服处理周期。
package servicetimeout

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	"github.com/runforyou-ai/cervi/internal/actions/customerservice"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/actions/serviceassignment"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

const (
	// ScanActionName 扫描各企业到达超时时长的开放客服处理周期。
	ScanActionName = "service_session.timeout_scan"
	// ProcessActionName 处理单个客服处理周期的超时提醒与回收。
	ProcessActionName = "service_session.timeout"
	// ScheduleKey 是超时扫描的定时计划键。
	ScheduleKey = "customer-service-timeout-scan"
)

// scanLimit 是单次扫描投递的周期上限，其余周期由下一次扫描继续处理。
const scanLimit = 200

// ProcessInput 定义单个客服处理周期的超时处理任务。
type ProcessInput struct {
	OrganizationID   string `json:"organizationId"`
	ServiceSessionID string `json:"serviceSessionId"`
}

// Enqueuer 投递单条处理任务和事务内的补分配任务。
type Enqueuer interface {
	servertask.Enqueuer
	servertask.TxEnqueuer
}

// Worker 执行客服处理周期的超时扫描与单条处理任务。
type Worker struct {
	db       *bun.DB
	enqueuer Enqueuer
}

// NewWorker 创建客服处理周期超时任务执行器。
func NewWorker(db *bun.DB, enqueuer Enqueuer) *Worker {
	return &Worker{db: db, enqueuer: enqueuer}
}

// Scan 跨企业读取到达提醒或回收时长的开放周期，按周期投递单条处理任务；同一周期在途时不重复投递。
func (w *Worker) Scan(ctx context.Context, _ struct{}) error {
	defaults := domain.DefaultServiceTimeouts()
	start := "GREATEST(ss.awaiting_reply_since, ss.assignee_assigned_at)"
	var rows []ProcessInput
	err := w.db.NewSelect().TableExpr("service_sessions AS ss").
		ColumnExpr("ss.organization_id, ss.id AS service_session_id").
		Join("LEFT JOIN customer_service_settings AS css ON css.organization_id = ss.organization_id").
		Join("LEFT JOIN organization_identities AS oi ON oi.organization_id = ss.organization_id AND oi.id = ss.assignee_identity_id").
		Where("ss.status = ? AND ss.awaiting_reply_since IS NOT NULL", domain.ServiceSessionStatusOpen).
		WhereGroup(" AND ", func(query *bun.SelectQuery) *bun.SelectQuery {
			return query.
				Where("oi.type = ? AND ss.reminded_at IS NULL AND "+start+" <= now() - make_interval(mins => COALESCE(css.response_reminder_minutes, ?))",
					domain.OrganizationIdentityTypeUser, defaults.ResponseReminderMinutes).
				WhereOr("oi.type = ? AND "+start+" <= now() - make_interval(mins => COALESCE(css.response_reclaim_minutes, ?))",
					domain.OrganizationIdentityTypeUser, defaults.ResponseReclaimMinutes).
				WhereOr("ss.assignee_identity_id IS NULL AND ss.reminded_at IS NULL AND ss.awaiting_reply_since <= now() - make_interval(mins => COALESCE(css.queue_reminder_minutes, ?))",
					defaults.QueueReminderMinutes)
		}).
		OrderExpr("ss.awaiting_reply_since ASC, ss.id ASC").
		Limit(scanLimit).
		Scan(ctx, &rows)
	if err != nil {
		return fmt.Errorf("scan service session timeouts: %w", err)
	}
	for _, row := range rows {
		if _, err := w.enqueuer.Enqueue(ctx, ProcessActionName, row, servertask.EnqueueOptions{MaxAttempts: 3, IdempotencyKey: "service-timeout:" + row.ServiceSessionID}); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("投递客服处理周期超时任务失败", "organization_id", row.OrganizationID, "service_session_id", row.ServiceSessionID, "error", err)
		}
	}
	return nil
}

// timeoutAction 表示单个周期当前应执行的超时动作。
type timeoutAction int

const (
	// actionNone 表示周期未到达任何超时时长或已提醒过。
	actionNone timeoutAction = iota
	// actionRemindAssignee 表示提醒负责人回复。
	actionRemindAssignee
	// actionReclaim 表示退回队列并重新分配。
	actionReclaim
	// actionRemindQueue 表示提醒队列对应的客服。
	actionRemindQueue
)

// dueAction 按周期当前状态和企业超时时长判断应执行的动作；assigneeType 为负责人身份类型，周期在队列中时为空。
func dueAction(session *servermodels.ServiceSession, assigneeType domain.OrganizationIdentityType, timeouts domain.ServiceTimeouts, now time.Time) timeoutAction {
	if domain.ServiceSessionStatus(session.Status) != domain.ServiceSessionStatusOpen || session.AwaitingReplySince == nil {
		return actionNone
	}
	if session.AssigneeIdentityID == nil {
		if session.RemindedAt == nil && !now.Before(session.AwaitingReplySince.Add(time.Duration(timeouts.QueueReminderMinutes)*time.Minute)) {
			return actionRemindQueue
		}
		return actionNone
	}
	if assigneeType != domain.OrganizationIdentityTypeUser {
		return actionNone
	}
	// 未响应计时从客户开始等待与负责人接手中较晚的时间起算。
	start := *session.AwaitingReplySince
	if session.AssigneeAssignedAt != nil && session.AssigneeAssignedAt.After(start) {
		start = *session.AssigneeAssignedAt
	}
	if !now.Before(start.Add(time.Duration(timeouts.ResponseReclaimMinutes) * time.Minute)) {
		return actionReclaim
	}
	if session.RemindedAt == nil && !now.Before(start.Add(time.Duration(timeouts.ResponseReminderMinutes)*time.Minute)) {
		return actionRemindAssignee
	}
	return actionNone
}

// Process 读取周期与企业超时时长后执行到期动作；每个动作在事务内锁定会话并复核条件，条件已不满足时直接结束。
func (w *Worker) Process(ctx context.Context, input ProcessInput) error {
	timeouts, err := customerservice.LoadServiceTimeouts(ctx, w.db, input.OrganizationID)
	if err != nil {
		return err
	}
	_, session, assigneeType, err := loadSession(ctx, w.db, input.OrganizationID, input.ServiceSessionID, false)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	switch dueAction(session, assigneeType, timeouts, time.Now()) {
	case actionReclaim:
		return w.reclaim(ctx, session, timeouts)
	case actionRemindAssignee, actionRemindQueue:
		return w.remind(ctx, input, timeouts)
	default:
		return nil
	}
}

// loadSession 读取客服处理周期与负责人身份类型；lock 为 true 时先锁定并返回所属会话与当前周期，会话已开始新的周期时返回 sql.ErrNoRows。
func loadSession(ctx context.Context, db bun.IDB, organizationID, serviceSessionID string, lock bool) (*servermodels.Conversation, *servermodels.ServiceSession, domain.OrganizationIdentityType, error) {
	session := &servermodels.ServiceSession{}
	if err := db.NewSelect().Model(session).
		Where("ss.organization_id = ? AND ss.id = ?", organizationID, serviceSessionID).
		Scan(ctx); err != nil {
		return nil, nil, "", err
	}
	var conversation *servermodels.Conversation
	if lock {
		lockedConversation, locked, err := chatstate.LockCustomerServiceSession(ctx, db, organizationID, session.ConversationID)
		if err != nil {
			return nil, nil, "", err
		}
		if locked.ID != session.ID {
			return nil, nil, "", sql.ErrNoRows
		}
		conversation, session = lockedConversation, locked
	}
	if session.AssigneeIdentityID == nil {
		return conversation, session, "", nil
	}
	var assigneeType domain.OrganizationIdentityType
	if err := db.NewSelect().Model((*servermodels.OrganizationIdentity)(nil)).Column("oi.type").
		Where("oi.organization_id = ? AND oi.id = ?", organizationID, *session.AssigneeIdentityID).
		Scan(ctx, &assigneeType); err != nil {
		return nil, nil, "", fmt.Errorf("load service session assignee type: %w", err)
	}
	return conversation, session, assigneeType, nil
}

// remind 在事务中锁定周期并复核仍需提醒后写入提醒时间，提交后提醒负责人或队列对应的工作中客服；没有收件人时本轮提醒保持未发出。
func (w *Worker) remind(ctx context.Context, input ProcessInput, timeouts domain.ServiceTimeouts) error {
	return realtime.RunInTx(ctx, w.db, func(ctx context.Context, tx bun.Tx) error {
		_, session, assigneeType, err := loadSession(ctx, tx, input.OrganizationID, input.ServiceSessionID, true)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		action := dueAction(session, assigneeType, timeouts, time.Now())
		if action != actionRemindAssignee && action != actionRemindQueue {
			return nil
		}
		reason := domain.ServiceAttentionResponseOverdue
		// 负责人提醒只发给负责人；队列提醒发给团队中或全企业工作中的客服。
		recipients := tx.NewSelect().TableExpr("organization_identities AS oi").
			ColumnExpr("u.id").
			Join("JOIN users AS u ON u.organization_id = oi.organization_id AND u.identity_id = oi.id").
			Where("oi.organization_id = ? AND oi.type = ?", session.OrganizationID, domain.OrganizationIdentityTypeUser)
		if action == actionRemindAssignee {
			recipients = recipients.Where("oi.id = ?", *session.AssigneeIdentityID)
		} else {
			reason = domain.ServiceAttentionQueueWaiting
			recipients = identityaction.ApplyCustomerHandlingConditions(recipients.Where("oi.work_status = ?", domain.WorkStatusWorking))
			if session.TeamID != nil {
				recipients = recipients.Where("EXISTS (SELECT 1 FROM team_members AS tm WHERE tm.organization_id = oi.organization_id AND tm.identity_id = oi.id AND tm.team_id = ?)", *session.TeamID)
			}
		}
		var userIDs []string
		if err := recipients.Scan(ctx, &userIDs); err != nil {
			return fmt.Errorf("load service session reminder recipients: %w", err)
		}
		// 没有收件人时不记录提醒，由之后的扫描在有人工作时补发。
		if len(userIDs) == 0 {
			return nil
		}
		if _, err := tx.NewUpdate().Model(session).
			Set("reminded_at = now()").
			Set("updated_at = now()").
			WherePK().Where("organization_id = ?", session.OrganizationID).
			Exec(ctx); err != nil {
			return fmt.Errorf("record service session reminder: %w", err)
		}
		for _, userID := range userIDs {
			realtime.Notify(ctx, realtime.UserServiceAttention(session.OrganizationID, userID, session.ConversationID, session.ID, reason))
		}
		slog.Info("客服处理周期等待超时，已提醒",
			"organization_id", session.OrganizationID, "conversation_id", session.ConversationID,
			"service_session_id", session.ID, "reason", reason, "recipient_count", len(userIDs))
		return nil
	})
}

// reclaim 把负责人超时未回复的周期退回原队列：先锁定排除原负责人后的候选成员、再锁会话并复核，写入退回事件后分配给候选成员，没有候选时留在队列并投递排除原负责人的分配任务；提交后告知原负责人并为其补分配。
func (w *Worker) reclaim(ctx context.Context, snapshot *servermodels.ServiceSession, timeouts domain.ServiceTimeouts) error {
	previousAssigneeID := *snapshot.AssigneeIdentityID
	return realtime.RunInTx(ctx, w.db, func(ctx context.Context, tx bun.Tx) error {
		member, err := serviceassignment.LockQueueMember(ctx, tx, snapshot.OrganizationID, snapshot.TeamID, previousAssigneeID)
		if err != nil {
			return err
		}
		conversation, session, assigneeType, err := loadSession(ctx, tx, snapshot.OrganizationID, snapshot.ID, true)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		// 负责人或队列在读取后已变化时由引起变化的操作负责后续处理；两个所属队列均为空表示同为公共队列。
		sameQueue := (session.TeamID == nil && snapshot.TeamID == nil) ||
			(session.TeamID != nil && snapshot.TeamID != nil && *session.TeamID == *snapshot.TeamID)
		if dueAction(session, assigneeType, timeouts, time.Now()) != actionReclaim ||
			*session.AssigneeIdentityID != previousAssigneeID || !sameQueue {
			return nil
		}
		previous := &servermodels.OrganizationIdentity{}
		if err := tx.NewSelect().Model(previous).Column("oi.id", "oi.display_name").
			Where("oi.organization_id = ? AND oi.id = ?", session.OrganizationID, previousAssigneeID).
			Scan(ctx); err != nil {
			return fmt.Errorf("load reclaimed assignee: %w", err)
		}
		target, err := chatstate.ServiceSessionQueueTarget(ctx, tx, session)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(domain.ServiceSessionReturnedEvent{
			ServiceSessionID: session.ID, FromIdentityID: previous.ID, FromDisplayName: previous.DisplayName,
			Target: target, Reason: domain.ServiceSessionReturnResponseTimeout,
		})
		if err != nil {
			return fmt.Errorf("encode service session returned event: %w", err)
		}
		eventType := string(domain.ConversationSystemEventServiceSessionReturned)
		if _, _, err := chatstate.AppendMessage(ctx, tx, conversation, &servermodels.Message{
			ID: uuid.NewV7().String(), OrganizationID: session.OrganizationID, ConversationID: session.ConversationID,
			ServiceSessionID: &session.ID, Type: string(domain.MessageTypeSystem), Visibility: string(domain.MessageVisibilityInternalOnly),
			SystemEventType: &eventType, SystemEventPayload: payload, OriginatedAt: time.Now().UTC(),
		}); err != nil {
			return fmt.Errorf("append service session returned event: %w", err)
		}
		if _, err := tx.NewUpdate().Model(session).
			Set("assignee_identity_id = NULL").
			Set("assignee_assigned_at = NULL").
			Set("reminded_at = NULL").
			Set("updated_at = now()").
			WherePK().Where("organization_id = ?", session.OrganizationID).
			Exec(ctx); err != nil {
			return fmt.Errorf("return service session to queue: %w", err)
		}
		session.AssigneeIdentityID, session.AssigneeAssignedAt, session.RemindedAt = nil, nil, nil
		// 没有候选时投递排除原负责人的分配任务，覆盖回收提交前已完成补分配的成员。
		if member != nil {
			if err := serviceassignment.Assign(ctx, tx, conversation, session, member); err != nil {
				return err
			}
		} else if err := serviceassignment.EnqueueAssign(ctx, tx, w.enqueuer, serviceassignment.AssignInput{
			OrganizationID: session.OrganizationID, ServiceSessionID: session.ID, ExcludeIdentityID: previousAssigneeID,
		}); err != nil {
			return err
		}
		if err := serviceassignment.EnqueueBackfill(ctx, tx, w.enqueuer, serviceassignment.BackfillInput{
			OrganizationID: session.OrganizationID, IdentityID: previousAssigneeID, ExcludeServiceSessionID: session.ID,
		}); err != nil {
			return err
		}
		// 读取原负责人账号，提醒其周期已退回队列。
		var previousUserID string
		if err := tx.NewSelect().Model((*servermodels.User)(nil)).Column("u.id").
			Where("u.organization_id = ? AND u.identity_id = ?", session.OrganizationID, previousAssigneeID).
			Scan(ctx, &previousUserID); err != nil {
			return fmt.Errorf("load reclaimed assignee user: %w", err)
		}
		realtime.Notify(ctx, realtime.UserServiceAttention(session.OrganizationID, previousUserID, session.ConversationID, session.ID, domain.ServiceAttentionReturned))
		slog.Info("负责人超时未回复，客服处理周期已退回队列",
			"organization_id", session.OrganizationID, "conversation_id", session.ConversationID,
			"service_session_id", session.ID, "previous_assignee_identity_id", previousAssigneeID,
			"target_kind", target.Kind, "reassigned", member != nil)
		return nil
	})
}
