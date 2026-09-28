//go:build server

package chatstate

import (
	"context"
	"fmt"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// AssignServiceSession 在调用方持有周期锁的事务中把周期交给负责人：首次负责时记录负责时间，负责人接手时间记为 now，清空排队与提醒时间，所属队列不变。
func AssignServiceSession(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession, identityID string, now time.Time) error {
	if _, err := db.NewUpdate().Model(session).
		Set("assignee_identity_id = ?", identityID).
		Set("assigned_at = COALESCE(assigned_at, ?)", now).
		Set("assignee_assigned_at = ?", now).
		Set("queued_at = NULL").
		Set("reminded_at = NULL").
		Set("updated_at = now()").
		WherePK().Where("organization_id = ?", session.OrganizationID).
		Exec(ctx); err != nil {
		return fmt.Errorf("assign service session: %w", err)
	}
	session.AssigneeIdentityID, session.AssigneeAssignedAt = &identityID, &now
	session.QueuedAt, session.RemindedAt = nil, nil
	if session.AssignedAt == nil {
		session.AssignedAt = &now
	}
	return nil
}

// ReturnServiceSessionToQueue 在调用方持有周期锁的事务中把周期退回 teamID 对应的队列：清空负责人与接手时间，从 now 起计入队列并清空提醒时间；teamID 为空表示公共队列，客户等待起点不变。
func ReturnServiceSessionToQueue(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession, teamID *string, now time.Time) error {
	if _, err := db.NewUpdate().Model(session).
		Set("assignee_identity_id = NULL").
		Set("assignee_assigned_at = NULL").
		Set("team_id = ?", teamID).
		Set("queued_at = ?", now).
		Set("reminded_at = NULL").
		Set("updated_at = now()").
		WherePK().Where("organization_id = ?", session.OrganizationID).
		Exec(ctx); err != nil {
		return fmt.Errorf("return service session to queue: %w", err)
	}
	session.AssigneeIdentityID, session.AssigneeAssignedAt, session.TeamID = nil, nil, teamID
	session.QueuedAt, session.RemindedAt = &now, nil
	return nil
}

// CloseServiceSession 在调用方持有周期锁的事务中关闭开放周期：记录关闭时间、关闭人与结束方式，清空客户等待、排队、提醒与确认请求时间，负责人保持不变。
func CloseServiceSession(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession, closedByIdentityID string, reason domain.ServiceSessionCloseReason, now time.Time) error {
	if _, err := db.NewUpdate().Model(session).
		Set("status = ?", domain.ServiceSessionStatusClosed).
		Set("status_changed_at = ?", now).
		Set("closed_at = ?", now).
		Set("closed_by_identity_id = ?", closedByIdentityID).
		Set("close_reason = ?", reason).
		Set("awaiting_reply_since = NULL").
		Set("queued_at = NULL").
		Set("reminded_at = NULL").
		Set("resolution_requested_at = NULL").
		Set("updated_at = now()").
		WherePK().Where("organization_id = ?", session.OrganizationID).
		Exec(ctx); err != nil {
		return fmt.Errorf("close service session: %w", err)
	}
	closeReason := string(reason)
	session.Status, session.StatusChangedAt, session.ClosedAt = string(domain.ServiceSessionStatusClosed), now, &now
	session.ClosedByIdentityID, session.CloseReason = &closedByIdentityID, &closeReason
	session.AwaitingReplySince, session.QueuedAt, session.RemindedAt, session.ResolutionRequestedAt = nil, nil, nil, nil
	return nil
}

// ReopenServiceSession 在调用方持有周期锁的事务中重新打开已关闭的周期：清空关闭记录并交给 identityID 负责。
func ReopenServiceSession(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession, identityID string, now time.Time) error {
	if _, err := db.NewUpdate().Model(session).
		Set("status = ?", domain.ServiceSessionStatusOpen).
		Set("status_changed_at = ?", now).
		Set("closed_at = NULL").
		Set("closed_by_identity_id = NULL").
		Set("close_reason = NULL").
		Set("updated_at = now()").
		WherePK().Where("organization_id = ?", session.OrganizationID).
		Exec(ctx); err != nil {
		return fmt.Errorf("reopen service session: %w", err)
	}
	session.Status, session.StatusChangedAt = string(domain.ServiceSessionStatusOpen), now
	session.ClosedAt, session.ClosedByIdentityID, session.CloseReason = nil, nil, nil
	return AssignServiceSession(ctx, db, session, identityID, now)
}
