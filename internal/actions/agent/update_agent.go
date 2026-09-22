//go:build server

package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	roleaction "github.com/runforyou-ai/cervi/internal/actions/role"
	teamaction "github.com/runforyou-ai/cervi/internal/actions/team"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateAgentAction 修改企业 AI 员工。
type UpdateAgentAction struct {
	db       *bun.DB
	returner ServiceSessionReturner
}

// NewUpdateAgentAction 创建 AI 员工修改操作。
func NewUpdateAgentAction(db *bun.DB, returner ServiceSessionReturner) *UpdateAgentAction {
	return &UpdateAgentAction{db: db, returner: returner}
}

// Execute 在事务中保存 AI 员工基本资料、接待开关、头像和工作状态；关闭接待开关时把其负责的开放客服周期退回原队列。
func (a *UpdateAgentAction) Execute(ctx context.Context, identity *servermodels.Identity, agentID string, input UpdateInput) (*Agent, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.DisplayName == "" {
		return nil, &common.FieldError{Fields: map[string]common.FieldCode{"displayName": ValidationDisplayNameRequired}}
	}
	if !domain.IdentityDisplayNameValid(input.DisplayName) {
		return nil, &common.FieldError{Fields: map[string]common.FieldCode{"displayName": ValidationDisplayNameInvalid}}
	}
	if !common.ValidUUID(agentID) {
		return nil, ErrNotFound
	}
	if input.WorkStatus != domain.WorkStatusWorking && input.WorkStatus != domain.WorkStatusAway && input.WorkStatus != domain.WorkStatusOffDuty {
		return nil, &common.FieldError{Fields: map[string]common.FieldCode{"workStatus": ValidationWorkStatusInvalid}}
	}
	var output *Agent
	var cancelledRunIDs []string
	err := realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		_, err := roleaction.ValidateAssignment(ctx, tx, identity.Organization.ID, input.RoleID, domain.OrganizationIdentityTypeAgent)
		if errors.Is(err, roleaction.ErrAssignmentInvalid) || errors.Is(err, roleaction.ErrAgentAdministrator) {
			return &common.FieldError{Fields: map[string]common.FieldCode{"roleId": ValidationRoleInvalid}}
		}
		if err != nil {
			return err
		}
		teamIDs, _, err := validateAndLoadTeams(ctx, tx, identity.Organization.ID, input.TeamIDs)
		if err != nil {
			return err
		}
		storedAgent := &servermodels.Agent{}
		err = tx.NewSelect().Model(storedAgent).
			Column("identity_id", "status").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", agentID).
			For("UPDATE").
			Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if domain.UserStatus(storedAgent.Status) == domain.UserStatusInactive && input.WorkStatus != domain.WorkStatusOffDuty {
			return &common.FieldError{Fields: map[string]common.FieldCode{"workStatus": ValidationWorkStatusUnavailable}}
		}
		// 先锁定身份，与以该身份为目标的入站路由和转交串行。
		locked, err := lockAgentIdentity(ctx, tx, identity.Organization.ID, storedAgent.IdentityID)
		if err != nil {
			return err
		}
		// 传入新头像时激活该图片，替换下来的旧头像交给清理任务。
		var nextAvatarFileID *string
		if input.AvatarFileID != "" {
			nextAvatarFileID, err = fileaction.ActivateLinkedImage(ctx, tx, identity.Organization.ID, domain.FilePurposeAgentAvatar, input.AvatarFileID, locked.AvatarFileID)
			if err != nil {
				return err
			}
			if err := fileaction.RetireLinkedImage(ctx, tx, identity.Organization.ID, locked.AvatarFileID, nextAvatarFileID); err != nil {
				return err
			}
		}
		var displayChanged bool
		err = tx.NewUpdate().Model((*servermodels.OrganizationIdentity)(nil)).
			Set("display_name = ?", input.DisplayName).
			Set("avatar_file_id = COALESCE(?, avatar_file_id)", nextAvatarFileID).
			Set("role_id = ?", input.RoleID).
			Set("handles_customers = ?", input.HandlesCustomers).
			Set("work_status_updated_at = CASE WHEN work_status <> ? THEN now() ELSE work_status_updated_at END", input.WorkStatus).
			Set("work_status = ?", input.WorkStatus).
			Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", storedAgent.IdentityID).
			Where("type = ?", domain.OrganizationIdentityTypeAgent).
			Returning("(old.display_name, old.avatar_file_id) IS DISTINCT FROM (new.display_name, new.avatar_file_id)").
			Scan(ctx, &displayChanged)
		if err != nil {
			return err
		}
		if err := teamaction.ReplaceIdentityTeams(ctx, tx, identity, storedAgent.IdentityID, teamIDs); err != nil {
			return err
		}
		if locked.HandlesCustomers && !input.HandlesCustomers {
			cancelledRunIDs, err = a.returner.ReturnServiceSessionsToQueue(ctx, tx, identity.Organization.ID, storedAgent.IdentityID, uuid.NewV7().String())
			if err != nil {
				return err
			}
		}
		// 名称或头像实际变化时，在资料写入与退回完成后推进展示该 AI 员工的会话版本；退回已锁定其负责的会话。
		if displayChanged {
			if err := chatstate.TouchIdentityConversations(ctx, tx, identity.Organization.ID, storedAgent.IdentityID); err != nil {
				return err
			}
		}
		output, err = loadAgent(ctx, tx, identity.Organization.ID, agentID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update agent: %w", err)
	}
	a.returner.CancelRunContexts(cancelledRunIDs)
	return output, nil
}
