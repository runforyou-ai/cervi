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

// ErrAssistantNotInConversation 表示助理不是会话的当前参与者，或当前成员不是其主人。
var ErrAssistantNotInConversation = errors.New("assistant is not in conversation")

// AssistantWorkspaceRecord 定义会话中一位助理的绑定电脑与工作区指定，WorkspaceID 为空表示尚未指定。
type AssistantWorkspaceRecord struct {
	AssistantIdentityID string `bun:"assistant_identity_id"`
	OwnerUserID         string `bun:"owner_user_id"`
	DeviceID            string `bun:"device_id"`
	DeviceName          string `bun:"device_name"`
	WorkspaceID         string `bun:"workspace_id"`
	WorkspaceLabel      string `bun:"workspace_label"`
}

// ConversationAssistantWorkspaceAction 管理会话中助理的工作区指定。
type ConversationAssistantWorkspaceAction struct {
	db *bun.DB
}

// NewConversationAssistantWorkspaceAction 创建会话助理工作区操作。
func NewConversationAssistantWorkspaceAction(db *bun.DB) *ConversationAssistantWorkspaceAction {
	return &ConversationAssistantWorkspaceAction{db: db}
}

// List 返回当前成员可阅读会话中各位助理的绑定电脑与工作区指定。
func (a *ConversationAssistantWorkspaceAction) List(ctx context.Context, identity *servermodels.Identity, conversationID string) ([]AssistantWorkspaceRecord, error) {
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
	records := make([]AssistantWorkspaceRecord, 0)
	if err := conversationAssistantQuery(a.db, identity.Organization.ID, conversationID).
		ColumnExpr("oi.id::text AS assistant_identity_id, a.owner_user_id::text AS owner_user_id, a.device_id::text AS device_id, d.name AS device_name").
		ColumnExpr("COALESCE(dw.id::text, '') AS workspace_id, COALESCE(dw.label, '') AS workspace_label").
		Join("JOIN devices AS d ON d.id = a.device_id AND d.organization_id = a.organization_id").
		Join("LEFT JOIN conversation_assistant_workspaces AS caw ON caw.organization_id = a.organization_id AND caw.conversation_id = cp.conversation_id AND caw.agent_id = a.id").
		Join("LEFT JOIN device_workspaces AS dw ON dw.id = caw.workspace_id AND dw.organization_id = caw.organization_id").
		OrderExpr("lower(oi.display_name) ASC, oi.id ASC").
		Scan(ctx, &records); err != nil {
		return nil, fmt.Errorf("list conversation assistant workspaces: %w", err)
	}
	return records, nil
}

// Set 由助理主人为会话中的助理指定其绑定电脑上的工作区，已有指定时替换，并推进会话版本通知各端重读。
func (a *ConversationAssistantWorkspaceAction) Set(ctx context.Context, identity *servermodels.Identity, conversationID, assistantIdentityID, workspaceID string) error {
	if !common.ValidUUID(conversationID) {
		return chatstate.ErrConversationNotFound
	}
	err := realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		// 锁序为助理记录、会话，与助理的启停、暂停和换电脑一致。
		if _, err := lockOwnAssistant(ctx, tx, identity, assistantIdentityID); err != nil {
			return err
		}
		member, err := lockActiveConversationMember(ctx, tx, identity, conversationID)
		if err != nil {
			return err
		}
		if err := SaveAssistantWorkspace(ctx, tx, identity, conversationID, assistantIdentityID, workspaceID); err != nil {
			return err
		}
		return chatstate.TouchConversation(ctx, tx, member.Conversation)
	})
	if err != nil {
		return fmt.Errorf("set conversation assistant workspace: %w", err)
	}
	slog.Info("会话已指定助理工作区", "organization_id", identity.Organization.ID, "conversation_id", conversationID,
		"assistant_identity_id", assistantIdentityID, "workspace_id", workspaceID)
	return nil
}

// Clear 由助理主人清除会话中助理的工作区指定并推进会话版本；尚未领取的运行由收敛扫描结束。
func (a *ConversationAssistantWorkspaceAction) Clear(ctx context.Context, identity *servermodels.Identity, conversationID, assistantIdentityID string) error {
	if !common.ValidUUID(conversationID) {
		return chatstate.ErrConversationNotFound
	}
	err := realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		// 锁序为助理记录、会话，与助理的启停、暂停和换电脑一致。
		if _, err := lockOwnAssistant(ctx, tx, identity, assistantIdentityID); err != nil {
			return err
		}
		member, err := lockActiveConversationMember(ctx, tx, identity, conversationID)
		if err != nil {
			return err
		}
		agentID, _, err := lockOwnConversationAssistant(ctx, tx, identity, conversationID, assistantIdentityID)
		if err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*servermodels.ConversationAssistantWorkspace)(nil)).
			Where("organization_id = ? AND conversation_id = ? AND agent_id = ?", identity.Organization.ID, conversationID, agentID).
			Exec(ctx); err != nil {
			return err
		}
		return chatstate.TouchConversation(ctx, tx, member.Conversation)
	})
	if err != nil {
		return fmt.Errorf("clear conversation assistant workspace: %w", err)
	}
	slog.Info("会话已清除助理工作区", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "assistant_identity_id", assistantIdentityID)
	return nil
}

// SaveAssistantWorkspace 在调用方已锁定的会话中，为当前成员名下的助理指定其绑定电脑上的工作区，已有指定时替换。
func SaveAssistantWorkspace(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, conversationID, assistantIdentityID, workspaceID string) error {
	agentID, deviceID, err := lockOwnConversationAssistant(ctx, tx, identity, conversationID, assistantIdentityID)
	if err != nil {
		return err
	}
	if !common.ValidUUID(workspaceID) {
		return ErrWorkspaceNotFound
	}
	exists, err := tx.NewSelect().Model((*servermodels.DeviceWorkspace)(nil)).
		Join("JOIN devices AS d ON d.id = dw.device_id AND d.organization_id = dw.organization_id").
		Where("dw.organization_id = ? AND dw.id = ? AND dw.device_id = ?", identity.Organization.ID, workspaceID, deviceID).
		Where("d.revoked_at IS NULL").
		For("SHARE OF d").Exists(ctx)
	if err != nil {
		return fmt.Errorf("load assistant workspace: %w", err)
	}
	if !exists {
		return ErrWorkspaceNotFound
	}
	if _, err := tx.NewInsert().Model(&servermodels.ConversationAssistantWorkspace{
		OrganizationID: identity.Organization.ID, ConversationID: conversationID, AgentID: agentID,
		WorkspaceID: workspaceID, AssignedByUserID: identity.User.ID,
	}).
		Column("organization_id", "conversation_id", "agent_id", "workspace_id", "assigned_by_user_id").
		On("CONFLICT (organization_id, conversation_id, agent_id) DO UPDATE").
		Set("workspace_id = EXCLUDED.workspace_id").
		Set("assigned_by_user_id = EXCLUDED.assigned_by_user_id").
		Set("updated_at = now()").
		Exec(ctx); err != nil {
		return fmt.Errorf("save conversation assistant workspace: %w", err)
	}
	return nil
}

// lockActiveConversationMember 锁定当前成员参与的活跃会话。
func lockActiveConversationMember(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, conversationID string) (chatstate.Member, error) {
	member, err := chatstate.LockMember(ctx, tx, identity, conversationID)
	if err != nil {
		return chatstate.Member{}, err
	}
	if member.Conversation.Status != string(domain.ConversationStatusActive) {
		return chatstate.Member{}, chatstate.ErrConversationNotFound
	}
	return member, nil
}

// lockOwnConversationAssistant 锁定会话中当前成员名下的助理，返回助理编号与其绑定电脑编号。
func lockOwnConversationAssistant(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, conversationID, assistantIdentityID string) (string, string, error) {
	if !common.ValidUUID(assistantIdentityID) {
		return "", "", ErrAssistantNotInConversation
	}
	var locked struct {
		AgentID  string `bun:"agent_id"`
		DeviceID string `bun:"device_id"`
	}
	err := conversationAssistantQuery(tx, identity.Organization.ID, conversationID).
		ColumnExpr("a.id::text AS agent_id, a.device_id::text AS device_id").
		Where("oi.id = ? AND a.owner_user_id = ?", assistantIdentityID, identity.User.ID).
		For("UPDATE OF a").
		Scan(ctx, &locked)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrAssistantNotInConversation
	}
	if err != nil {
		return "", "", fmt.Errorf("lock conversation assistant: %w", err)
	}
	return locked.AgentID, locked.DeviceID, nil
}

// lockOwnAssistant 锁定当前成员名下的助理记录，返回助理编号。
func lockOwnAssistant(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, assistantIdentityID string) (string, error) {
	if !common.ValidUUID(assistantIdentityID) {
		return "", ErrAssistantNotInConversation
	}
	var agentID string
	err := tx.NewSelect().TableExpr("agents AS a").ColumnExpr("a.id::text").
		Join("JOIN organization_identities AS oi ON oi.organization_id = a.organization_id AND oi.id = a.identity_id AND oi.type = ?", domain.OrganizationIdentityTypeAssistant).
		Where("a.organization_id = ? AND a.identity_id = ? AND a.owner_user_id = ?", identity.Organization.ID, assistantIdentityID, identity.User.ID).
		For("UPDATE OF a").
		Scan(ctx, &agentID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrAssistantNotInConversation
	}
	if err != nil {
		return "", fmt.Errorf("lock assistant: %w", err)
	}
	return agentID, nil
}

// conversationAssistantQuery 构造会话中当前参与的助理查询。
func conversationAssistantQuery(db bun.IDB, organizationID, conversationID string) *bun.SelectQuery {
	return db.NewSelect().TableExpr("conversation_participants AS cp").
		Join("JOIN chat_subjects AS cs ON cs.organization_id = cp.organization_id AND cs.id = cp.subject_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("JOIN organization_identities AS oi ON oi.organization_id = cs.organization_id AND oi.id = cs.source_id AND oi.type = ?", domain.OrganizationIdentityTypeAssistant).
		Join("JOIN agents AS a ON a.organization_id = oi.organization_id AND a.identity_id = oi.id").
		Where("cp.organization_id = ? AND cp.conversation_id = ? AND cp.left_at IS NULL", organizationID, conversationID)
}
