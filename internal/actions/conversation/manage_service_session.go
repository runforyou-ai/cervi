//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/storage/server/pgerr"
	"github.com/uptrace/bun"
)

// ServiceSessionAgentRunCoordinator 在客服事务内收敛原负责人的 Agent 执行。
type ServiceSessionAgentRunCoordinator interface {
	CancelForServiceSession(context.Context, bun.IDB, string, string, string, domain.AgentRunErrorCode) ([]string, error)
	CancelRunContexts([]string)
}

// ClaimServiceSessionAction 领取或接管客户会话最新处理周期。
type ClaimServiceSessionAction struct {
	db          *bun.DB
	coordinator ServiceSessionAgentRunCoordinator
}

// NewClaimServiceSessionAction 创建客服处理周期领取操作。
func NewClaimServiceSessionAction(db *bun.DB, coordinator ServiceSessionAgentRunCoordinator) *ClaimServiceSessionAction {
	return &ClaimServiceSessionAction{db: db, coordinator: coordinator}
}

// Execute 把未关闭处理周期负责人设置为当前身份。
func (a *ClaimServiceSessionAction) Execute(ctx context.Context, identity *servermodels.Identity, conversationID string) (ServiceSessionResult, error) {
	conversationID, valid := common.NormalizeUUID(conversationID)
	if !valid {
		return ServiceSessionResult{}, &ValidationError{Fields: map[string]ValidationCode{"conversationId": ValidationConversationIDInvalid}}
	}
	var output ServiceSessionResult
	var cancelledRunIDs []string
	var cancelledSession *servermodels.ServiceSession
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		session, err := lockOpenServiceSession(ctx, tx, identity.Organization.ID, conversationID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if session.AssigneeIdentityID == nil || *session.AssigneeIdentityID != identity.OrganizationIdentity.ID {
			if session.AssigneeIdentityID != nil {
				cancelledRunIDs, err = a.coordinator.CancelForServiceSession(
					ctx, tx, session.OrganizationID, session.ConversationID,
					*session.AssigneeIdentityID, domain.AgentRunErrorCodeAssigneeChanged,
				)
				if err != nil {
					return err
				}
				cancelledSession = session
			}
			if _, err := tx.NewUpdate().Model(session).
				Set("assignee_identity_id = ?", identity.OrganizationIdentity.ID).
				Set("assigned_at = COALESCE(assigned_at, ?)", now).
				Set("updated_at = now()").
				WherePK().
				Where("organization_id = ?", identity.Organization.ID).
				Exec(ctx); err != nil {
				return err
			}
			assigneeIdentityID := identity.OrganizationIdentity.ID
			session.AssigneeIdentityID = &assigneeIdentityID
		}
		output = serviceSessionResult(session, &identity.OrganizationIdentity)
		return nil
	})
	if err != nil {
		return ServiceSessionResult{}, fmt.Errorf("claim service session: %w", err)
	}
	finishServiceSessionAgentCancellation(a.coordinator, cancelledRunIDs, cancelledSession, domain.AgentRunErrorCodeAssigneeChanged)
	return output, nil
}

// TransferServiceSessionAction 把当前负责的处理周期转给另一位客服。
type TransferServiceSessionAction struct {
	db          *bun.DB
	coordinator ServiceSessionAgentRunCoordinator
	scheduler   CustomerAgentMessageScheduler
}

// NewTransferServiceSessionAction 创建客服处理周期转交操作。
func NewTransferServiceSessionAction(db *bun.DB, coordinator ServiceSessionAgentRunCoordinator, scheduler CustomerAgentMessageScheduler) *TransferServiceSessionAction {
	return &TransferServiceSessionAction{db: db, coordinator: coordinator, scheduler: scheduler}
}

// Execute 校验当前负责人和目标客服后保存转交。
func (a *TransferServiceSessionAction) Execute(ctx context.Context, identity *servermodels.Identity, input TransferServiceSessionInput) (ServiceSessionResult, error) {
	var conversationValid, assigneeValid bool
	input.ConversationID, conversationValid = common.NormalizeUUID(input.ConversationID)
	input.AssigneeIdentityID, assigneeValid = common.NormalizeUUID(input.AssigneeIdentityID)
	fields := map[string]ValidationCode{}
	if !conversationValid {
		fields["conversationId"] = ValidationConversationIDInvalid
	}
	if !assigneeValid || input.AssigneeIdentityID == identity.OrganizationIdentity.ID {
		fields["assigneeIdentityId"] = ValidationTargetIdentityIDInvalid
	}
	if len(fields) > 0 {
		return ServiceSessionResult{}, &ValidationError{Fields: fields}
	}
	var output ServiceSessionResult
	var cancelledRunIDs []string
	var cancelledSession *servermodels.ServiceSession
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		session, err := lockOpenServiceSession(ctx, tx, identity.Organization.ID, input.ConversationID)
		if err != nil {
			return err
		}
		if session.AssigneeIdentityID == nil || *session.AssigneeIdentityID != identity.OrganizationIdentity.ID {
			return &ConflictError{Reason: ConflictReasonServiceSessionOwned}
		}
		channelType, err := loadServiceSessionChannelType(ctx, tx, session)
		if err != nil {
			return err
		}
		target, err := identityaction.LockActiveCustomerServiceIdentity(ctx, tx, identity.Organization.ID, input.AssigneeIdentityID)
		if errors.Is(err, sql.ErrNoRows) {
			return &ValidationError{Fields: map[string]ValidationCode{"assigneeIdentityId": ValidationTargetIdentityIDInvalid}}
		}
		if err != nil {
			return err
		}
		if domain.OrganizationIdentityType(target.Type) == domain.OrganizationIdentityTypeAgent && !domain.ChannelSupportsAgentAssignee(channelType) {
			return &ValidationError{Fields: map[string]ValidationCode{"assigneeIdentityId": ValidationTargetIdentityIDInvalid}}
		}
		cancelledRunIDs, err = a.coordinator.CancelForServiceSession(
			ctx, tx, session.OrganizationID, session.ConversationID,
			*session.AssigneeIdentityID, domain.AgentRunErrorCodeAssigneeChanged,
		)
		if err != nil {
			return err
		}
		cancelledSession = session
		if _, err := tx.NewUpdate().Model(session).
			Set("assignee_identity_id = ?", target.ID).
			Set("assigned_at = COALESCE(assigned_at, ?)", time.Now().UTC()).
			Set("updated_at = now()").
			WherePK().
			Where("organization_id = ?", identity.Organization.ID).
			Exec(ctx); err != nil {
			return err
		}
		session.AssigneeIdentityID = &target.ID
		if domain.OrganizationIdentityType(target.Type) == domain.OrganizationIdentityTypeAgent {
			kind, messageID, err := loadServiceSessionLastMessageSender(ctx, tx, session)
			if err != nil {
				return err
			}
			if kind == domain.ChatSubjectKindContact {
				if a.scheduler == nil {
					return errors.New("customer agent scheduler is unavailable")
				}
				scheduled, err := a.scheduler.ScheduleCustomerAuto(
					ctx, tx, session.OrganizationID, session.ConversationID, session.ID, messageID,
				)
				if err != nil {
					return err
				}
				if !scheduled {
					return &ValidationError{Fields: map[string]ValidationCode{"assigneeIdentityId": ValidationTargetIdentityIDInvalid}}
				}
			}
		}
		output = serviceSessionResult(session, target)
		return nil
	})
	if err != nil {
		return ServiceSessionResult{}, fmt.Errorf("transfer service session: %w", err)
	}
	finishServiceSessionAgentCancellation(a.coordinator, cancelledRunIDs, cancelledSession, domain.AgentRunErrorCodeAssigneeChanged)
	return output, nil
}

// loadServiceSessionLastMessageSender 读取当前处理周期最后消息的发送主体类型。
func loadServiceSessionLastMessageSender(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession) (domain.ChatSubjectKind, string, error) {
	row := struct {
		MessageID string `bun:"message_id"`
		Kind      string `bun:"kind"`
	}{}
	err := db.NewSelect().
		TableExpr("messages AS msg").
		ColumnExpr("msg.id AS message_id, cs.kind AS kind").
		Join("JOIN conversation_participants AS cp ON cp.id = msg.sender_participant_id AND cp.organization_id = msg.organization_id AND cp.conversation_id = msg.conversation_id").
		Join("JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id").
		Where("msg.organization_id = ?", session.OrganizationID).
		Where("msg.conversation_id = ?", session.ConversationID).
		Where("msg.service_session_id = ?", session.ID).
		Where("msg.id = ?", session.LastMessageID).
		Where("msg.deleted_at IS NULL").
		Scan(ctx, &row)
	if err != nil {
		return "", "", fmt.Errorf("load service session last message sender: %w", err)
	}
	kind := domain.ChatSubjectKind(row.Kind)
	if kind != domain.ChatSubjectKindContact && kind != domain.ChatSubjectKindOrganizationIdentity {
		return "", "", ErrDataInvariant
	}
	return kind, row.MessageID, nil
}

// CloseServiceSessionAction 关闭客户会话最新处理周期。
type CloseServiceSessionAction struct {
	db          *bun.DB
	coordinator ServiceSessionAgentRunCoordinator
}

// NewCloseServiceSessionAction 创建客服处理周期关闭操作。
func NewCloseServiceSessionAction(db *bun.DB, coordinator ServiceSessionAgentRunCoordinator) *CloseServiceSessionAction {
	return &CloseServiceSessionAction{db: db, coordinator: coordinator}
}

// Execute 关闭公共队列或当前身份负责的处理周期。
func (a *CloseServiceSessionAction) Execute(ctx context.Context, identity *servermodels.Identity, conversationID string) (ServiceSessionResult, error) {
	conversationID, valid := common.NormalizeUUID(conversationID)
	if !valid {
		return ServiceSessionResult{}, &ValidationError{Fields: map[string]ValidationCode{"conversationId": ValidationConversationIDInvalid}}
	}
	var output ServiceSessionResult
	var cancelledRunIDs []string
	var cancelledSession *servermodels.ServiceSession
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		session, err := lockOpenServiceSession(ctx, tx, identity.Organization.ID, conversationID)
		if err != nil {
			return err
		}
		if session.AssigneeIdentityID != nil && *session.AssigneeIdentityID != identity.OrganizationIdentity.ID {
			return &ConflictError{Reason: ConflictReasonServiceSessionOwned}
		}
		if session.AssigneeIdentityID != nil {
			cancelledRunIDs, err = a.coordinator.CancelForServiceSession(
				ctx, tx, session.OrganizationID, session.ConversationID,
				*session.AssigneeIdentityID, domain.AgentRunErrorCodeSessionClosed,
			)
			if err != nil {
				return err
			}
			cancelledSession = session
		}
		now := time.Now().UTC()
		if _, err := tx.NewUpdate().Model(session).
			Set("status = ?", domain.ServiceSessionStatusClosed).
			Set("status_changed_at = ?", now).
			Set("closed_at = ?", now).
			Set("closed_by_identity_id = ?", identity.OrganizationIdentity.ID).
			Set("updated_at = now()").
			WherePK().
			Where("organization_id = ?", identity.Organization.ID).
			Exec(ctx); err != nil {
			return err
		}
		session.Status = string(domain.ServiceSessionStatusClosed)
		session.ClosedAt = &now
		var assignee *servermodels.OrganizationIdentity
		if session.AssigneeIdentityID != nil {
			assignee, err = loadAssigneeIdentity(ctx, tx, identity.Organization.ID, *session.AssigneeIdentityID)
			if err != nil {
				return err
			}
		}
		output = serviceSessionResult(session, assignee)
		return nil
	})
	if err != nil {
		return ServiceSessionResult{}, fmt.Errorf("close service session: %w", err)
	}
	finishServiceSessionAgentCancellation(a.coordinator, cancelledRunIDs, cancelledSession, domain.AgentRunErrorCodeSessionClosed)
	return output, nil
}

// ReopenServiceSessionAction 重新打开已关闭的客户会话处理周期。
type ReopenServiceSessionAction struct{ db *bun.DB }

// NewReopenServiceSessionAction 创建客服处理周期重新打开操作。
func NewReopenServiceSessionAction(db *bun.DB) *ReopenServiceSessionAction {
	return &ReopenServiceSessionAction{db: db}
}

// Execute 重新打开最新处理周期并分配给当前身份。
func (a *ReopenServiceSessionAction) Execute(ctx context.Context, identity *servermodels.Identity, conversationID string) (ServiceSessionResult, error) {
	conversationID, valid := common.NormalizeUUID(conversationID)
	if !valid {
		return ServiceSessionResult{}, &ValidationError{Fields: map[string]ValidationCode{"conversationId": ValidationConversationIDInvalid}}
	}
	var output ServiceSessionResult
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.Validate(ctx, tx, identity); err != nil {
			return err
		}
		session, err := lockLatestCustomerServiceSession(ctx, tx, identity.Organization.ID, conversationID)
		if err != nil {
			return err
		}
		if domain.ServiceSessionStatus(session.Status) == domain.ServiceSessionStatusOpen {
			return &ConflictError{Reason: ConflictReasonServiceSessionAlreadyOpen}
		}
		if domain.ServiceSessionStatus(session.Status) != domain.ServiceSessionStatusClosed {
			return ErrDataInvariant
		}
		now := time.Now().UTC()
		if _, err := tx.NewUpdate().Model(session).
			Set("status = ?", domain.ServiceSessionStatusOpen).
			Set("assignee_identity_id = ?", identity.OrganizationIdentity.ID).
			Set("assigned_at = COALESCE(assigned_at, ?)", now).
			Set("status_changed_at = ?", now).
			Set("closed_at = NULL").
			Set("closed_by_identity_id = NULL").
			Set("updated_at = now()").
			WherePK().
			Where("organization_id = ?", identity.Organization.ID).
			Exec(ctx); err != nil {
			return err
		}
		assigneeIdentityID := identity.OrganizationIdentity.ID
		session.Status = string(domain.ServiceSessionStatusOpen)
		session.AssigneeIdentityID = &assigneeIdentityID
		session.ClosedAt = nil
		output = serviceSessionResult(session, &identity.OrganizationIdentity)
		return nil
	})
	if err != nil {
		if pgerr.UniqueViolationOn(err, "service_sessions_organization_conversation_open_unique") {
			err = &ConflictError{Reason: ConflictReasonServiceSessionAlreadyOpen}
		}
		return ServiceSessionResult{}, fmt.Errorf("reopen service session: %w", err)
	}
	return output, nil
}

// lockOpenServiceSession 锁定客户会话最新且未关闭的客服处理周期。
func lockOpenServiceSession(ctx context.Context, db bun.IDB, organizationID, conversationID string) (*servermodels.ServiceSession, error) {
	session, err := lockLatestCustomerServiceSession(ctx, db, organizationID, conversationID)
	if err != nil {
		return nil, err
	}
	if domain.ServiceSessionStatus(session.Status) != domain.ServiceSessionStatusOpen {
		return nil, &ConflictError{Reason: ConflictReasonServiceSessionNotReplyable}
	}
	return session, nil
}

// lockLatestCustomerServiceSession 校验客户会话并锁定最新处理周期。
func lockLatestCustomerServiceSession(ctx context.Context, db bun.IDB, organizationID, conversationID string) (*servermodels.ServiceSession, error) {
	if _, err := loadCustomerConversationForReply(ctx, db, organizationID, conversationID); err != nil {
		return nil, err
	}
	session, err := lockLatestServiceSessionForReply(ctx, db, organizationID, conversationID)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// loadServiceSessionChannelType 返回客服处理周期所属的消息渠道类型。
func loadServiceSessionChannelType(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession) (domain.ChannelType, error) {
	var channelType domain.ChannelType
	err := db.NewSelect().TableExpr("contact_channel_identities AS cci").
		ColumnExpr("c.type").
		Join("JOIN channels AS c ON c.id = cci.channel_id AND c.organization_id = cci.organization_id").
		Where("cci.id = ?", session.ContactChannelIdentityID).
		Where("cci.organization_id = ?", session.OrganizationID).
		Scan(ctx, &channelType)
	return channelType, err
}

// loadAssigneeIdentity 读取处理周期负责人身份。
func loadAssigneeIdentity(ctx context.Context, db bun.IDB, organizationID, identityID string) (*servermodels.OrganizationIdentity, error) {
	assignee := &servermodels.OrganizationIdentity{}
	err := db.NewSelect().Model(assignee).
		Column("oi.id", "oi.type", "oi.display_name", "oi.avatar_file_id").
		Where("oi.organization_id = ?", organizationID).
		Where("oi.id = ?", identityID).
		Scan(ctx)
	return assignee, err
}

// serviceSessionResult 转换客服处理周期命令结果。
func serviceSessionResult(session *servermodels.ServiceSession, assignee *servermodels.OrganizationIdentity) ServiceSessionResult {
	var resultAssignee *ServiceSessionAssignee
	if assignee != nil {
		resultAssignee = &ServiceSessionAssignee{IdentityID: assignee.ID, Type: domain.OrganizationIdentityType(assignee.Type), DisplayName: assignee.DisplayName, AvatarFileID: assignee.AvatarFileID}
	}
	return ServiceSessionResult{ID: session.ID, Status: domain.ServiceSessionStatus(session.Status), Assignee: resultAssignee, ClosedAt: session.ClosedAt}
}

// finishServiceSessionAgentCancellation 在事务提交后取消本进程中的模型调用并记录结果。
func finishServiceSessionAgentCancellation(coordinator ServiceSessionAgentRunCoordinator, runIDs []string, session *servermodels.ServiceSession, reason domain.AgentRunErrorCode) {
	if len(runIDs) == 0 {
		return
	}
	coordinator.CancelRunContexts(runIDs)
	for _, runID := range runIDs {
		slog.Info("已取消客服会话 Agent 运行",
			"agent_run_id", runID,
			"service_session_id", session.ID,
			"conversation_id", session.ConversationID,
			"reason", reason,
		)
	}
}
