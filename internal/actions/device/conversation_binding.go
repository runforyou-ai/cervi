//go:build server

package device

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ErrBindingUnsupported 表示会话类型或当前成员不支持绑定设备。
var ErrBindingUnsupported = errors.New("conversation does not support device binding")

// BindingRecord 定义会话当前绑定的设备与工作区。
type BindingRecord struct {
	DeviceID       string `bun:"device_id"`
	DeviceName     string `bun:"device_name"`
	WorkspaceID    string `bun:"workspace_id"`
	WorkspaceLabel string `bun:"workspace_label"`
}

// ConversationBindingAction 管理 AI 单聊的设备与工作区绑定。
type ConversationBindingAction struct {
	db *bun.DB
}

// NewConversationBindingAction 创建会话设备绑定操作。
func NewConversationBindingAction(db *bun.DB) *ConversationBindingAction {
	return &ConversationBindingAction{db: db}
}

// Get 返回当前成员可阅读的 AI 单聊绑定，没有绑定时返回 nil。
func (a *ConversationBindingAction) Get(ctx context.Context, identity *servermodels.Identity, conversationID string) (*BindingRecord, error) {
	if !common.ValidUUID(conversationID) {
		return nil, chatstate.ErrConversationNotFound
	}
	readable, err := chatstate.MemberQuery(a.db, identity, conversationID).Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("check conversation member: %w", err)
	}
	if !readable {
		return nil, chatstate.ErrConversationNotFound
	}
	return loadBinding(ctx, a.db, identity.Organization.ID, conversationID)
}

// Bind 由 AI 单聊的成员把会话绑定到本人未撤销设备上的工作区，已有绑定时替换，并推进会话版本通知各端重读。
func (a *ConversationBindingAction) Bind(ctx context.Context, identity *servermodels.Identity, conversationID, workspaceID string) (*BindingRecord, error) {
	if !common.ValidUUID(conversationID) {
		return nil, chatstate.ErrConversationNotFound
	}
	if !common.ValidUUID(workspaceID) {
		return nil, ErrWorkspaceNotFound
	}
	var binding *BindingRecord
	err := realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		conversation, err := lockBindableConversation(ctx, tx, identity, conversationID)
		if err != nil {
			return err
		}
		var deviceID string
		err = tx.NewSelect().Model((*servermodels.DeviceWorkspace)(nil)).Column("dw.device_id").
			Join("JOIN devices AS d ON d.id = dw.device_id AND d.organization_id = dw.organization_id").
			Where("dw.organization_id = ? AND dw.id = ?", identity.Organization.ID, workspaceID).
			Where("d.user_id = ? AND d.revoked_at IS NULL", identity.User.ID).
			For("SHARE OF d").Scan(ctx, &deviceID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrWorkspaceNotFound
		}
		if err != nil {
			return fmt.Errorf("load binding workspace: %w", err)
		}
		if _, err := tx.NewInsert().Model(&servermodels.ConversationDeviceBinding{
			OrganizationID: identity.Organization.ID, ConversationID: conversationID, DeviceID: deviceID, WorkspaceID: workspaceID,
			PeerTriggerCapability: domain.PeerTriggerCapabilityOff, BoundByUserID: identity.User.ID,
		}).
			Column("organization_id", "conversation_id", "device_id", "workspace_id", "peer_trigger_capability", "bound_by_user_id").
			On("CONFLICT (organization_id, conversation_id) DO UPDATE").
			Set("device_id = EXCLUDED.device_id").
			Set("workspace_id = EXCLUDED.workspace_id").
			Set("bound_by_user_id = EXCLUDED.bound_by_user_id").
			Set("updated_at = now()").
			Exec(ctx); err != nil {
			return fmt.Errorf("save conversation device binding: %w", err)
		}
		binding, err = loadBinding(ctx, tx, identity.Organization.ID, conversationID)
		if err != nil {
			return err
		}
		return chatstate.TouchConversation(ctx, tx, conversation)
	})
	if err != nil {
		return nil, err
	}
	slog.Info("会话已绑定设备工作区", "organization_id", identity.Organization.ID, "conversation_id", conversationID,
		"device_id", binding.DeviceID, "workspace_id", workspaceID)
	return binding, nil
}

// Unbind 由 AI 单聊的成员解除会话的设备绑定并推进会话版本；尚未领取的设备运行由收敛扫描标记失败。
func (a *ConversationBindingAction) Unbind(ctx context.Context, identity *servermodels.Identity, conversationID string) error {
	if !common.ValidUUID(conversationID) {
		return chatstate.ErrConversationNotFound
	}
	err := realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		conversation, err := lockBindableConversation(ctx, tx, identity, conversationID)
		if err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*servermodels.ConversationDeviceBinding)(nil)).
			Where("organization_id = ? AND conversation_id = ?", identity.Organization.ID, conversationID).
			Exec(ctx); err != nil {
			return err
		}
		return chatstate.TouchConversation(ctx, tx, conversation)
	})
	if err != nil {
		return fmt.Errorf("unbind conversation device: %w", err)
	}
	slog.Info("会话已解除设备绑定", "organization_id", identity.Organization.ID, "conversation_id", conversationID)
	return nil
}

// lockBindableConversation 锁定并返回当前成员参与的活跃 AI 单聊。
func lockBindableConversation(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, conversationID string) (*servermodels.Conversation, error) {
	member, err := chatstate.LockMember(ctx, tx, identity, conversationID)
	if err != nil {
		return nil, err
	}
	if member.Conversation.Type != string(domain.ConversationTypeAgent) || member.Conversation.Status != string(domain.ConversationStatusActive) {
		return nil, ErrBindingUnsupported
	}
	return member.Conversation, nil
}

// loadBinding 读取会话当前的设备与工作区绑定，没有绑定时返回 nil。
func loadBinding(ctx context.Context, db bun.IDB, organizationID, conversationID string) (*BindingRecord, error) {
	binding := &BindingRecord{}
	err := db.NewSelect().Model((*servermodels.ConversationDeviceBinding)(nil)).
		ColumnExpr("cdb.device_id, d.name AS device_name, cdb.workspace_id, dw.label AS workspace_label").
		Join("JOIN devices AS d ON d.id = cdb.device_id AND d.organization_id = cdb.organization_id").
		Join("JOIN device_workspaces AS dw ON dw.id = cdb.workspace_id AND dw.organization_id = cdb.organization_id").
		Where("cdb.organization_id = ? AND cdb.conversation_id = ?", organizationID, conversationID).
		Scan(ctx, binding)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load conversation device binding: %w", err)
	}
	return binding, nil
}
