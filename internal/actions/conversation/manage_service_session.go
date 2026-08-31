//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/storage/server/pgerr"
	"github.com/uptrace/bun"
)

// ClaimServiceSessionAction 领取或接管客户会话最新处理周期。
type ClaimServiceSessionAction struct{ db *bun.DB }

// NewClaimServiceSessionAction 创建客服处理周期领取操作。
func NewClaimServiceSessionAction(db *bun.DB) *ClaimServiceSessionAction {
	return &ClaimServiceSessionAction{db: db}
}

// Execute 把未关闭处理周期负责人设置为当前身份。
func (a *ClaimServiceSessionAction) Execute(ctx context.Context, identity *servermodels.Identity, conversationID string) (ServiceSessionResult, error) {
	conversationID, valid := common.NormalizeUUID(conversationID)
	if !valid {
		return ServiceSessionResult{}, &ValidationError{Fields: map[string]ValidationCode{"conversationId": ValidationConversationIDInvalid}}
	}
	var output ServiceSessionResult
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
	return output, nil
}

// TransferServiceSessionAction 把当前负责的处理周期转给另一位客服。
type TransferServiceSessionAction struct{ db *bun.DB }

// NewTransferServiceSessionAction 创建客服处理周期转交操作。
func NewTransferServiceSessionAction(db *bun.DB) *TransferServiceSessionAction {
	return &TransferServiceSessionAction{db: db}
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
		target, err := loadActiveCustomerServiceIdentity(ctx, tx, identity.Organization.ID, input.AssigneeIdentityID)
		if errors.Is(err, sql.ErrNoRows) {
			return &ValidationError{Fields: map[string]ValidationCode{"assigneeIdentityId": ValidationTargetIdentityIDInvalid}}
		}
		if err != nil {
			return err
		}
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
		output = serviceSessionResult(session, target)
		return nil
	})
	if err != nil {
		return ServiceSessionResult{}, fmt.Errorf("transfer service session: %w", err)
	}
	return output, nil
}

// CloseServiceSessionAction 关闭客户会话最新处理周期。
type CloseServiceSessionAction struct{ db *bun.DB }

// NewCloseServiceSessionAction 创建客服处理周期关闭操作。
func NewCloseServiceSessionAction(db *bun.DB) *CloseServiceSessionAction {
	return &CloseServiceSessionAction{db: db}
}

// Execute 关闭公共队列或当前身份负责的处理周期。
func (a *CloseServiceSessionAction) Execute(ctx context.Context, identity *servermodels.Identity, conversationID string) (ServiceSessionResult, error) {
	conversationID, valid := common.NormalizeUUID(conversationID)
	if !valid {
		return ServiceSessionResult{}, &ValidationError{Fields: map[string]ValidationCode{"conversationId": ValidationConversationIDInvalid}}
	}
	var output ServiceSessionResult
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

// loadActiveCustomerServiceIdentity 读取有效的真人或 AI 客服身份。
func loadActiveCustomerServiceIdentity(ctx context.Context, db bun.IDB, organizationID, identityID string) (*servermodels.OrganizationIdentity, error) {
	target := &servermodels.OrganizationIdentity{}
	err := db.NewSelect().Model(target).
		Column("oi.id", "oi.type", "oi.display_name", "oi.avatar_file_id").
		Join("JOIN roles AS r ON r.id = oi.role_id AND r.organization_id = oi.organization_id AND r.kind = ?", domain.RoleKindCustomerService).
		Where("oi.organization_id = ?", organizationID).
		Where("oi.id = ?", identityID).
		Where("((oi.type = ? AND EXISTS (SELECT 1 FROM users AS u WHERE u.organization_id = oi.organization_id AND u.identity_id = oi.id AND u.status = ?)) OR (oi.type = ? AND EXISTS (SELECT 1 FROM agents AS a WHERE a.organization_id = oi.organization_id AND a.identity_id = oi.id AND a.status = ?)))", domain.OrganizationIdentityTypeUser, domain.UserStatusActive, domain.OrganizationIdentityTypeAgent, domain.UserStatusActive).
		For("KEY SHARE OF oi").
		Scan(ctx)
	return target, err
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
