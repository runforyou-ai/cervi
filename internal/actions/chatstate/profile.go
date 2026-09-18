//go:build server

package chatstate

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// TouchIdentityConversations 推进展示指定企业身份名称、头像或账号状态的会话版本，网站访客页面展示成员与 AI 员工的名称和头像，同时通知访客目录受众：含已退出的参与记录、当前负责的客户会话、以其为 Agent 或创建人的 Copilot 线程所属客户会话，以及其运行记录和待执行队列所在的会话；调用方在完成资料对象的全部写入后调用。
func TouchIdentityConversations(ctx context.Context, db bun.IDB, organizationID, identityID string) error {
	participants := db.NewSelect().TableExpr("conversation_participants AS cp").
		Column("cp.conversation_id").
		Join("JOIN chat_subjects AS cs ON cs.organization_id = cp.organization_id AND cs.id = cp.subject_id").
		Where("cp.organization_id = ?", organizationID).
		Where("cs.kind = ? AND cs.source_id = ?", domain.ChatSubjectKindOrganizationIdentity, identityID)
	assigned := db.NewSelect().TableExpr("customer_conversations AS cc").
		Column("cc.conversation_id").
		Join("JOIN service_sessions AS ss ON ss.organization_id = cc.organization_id AND ss.id = cc.current_service_session_id").
		Where("cc.organization_id = ? AND ss.assignee_identity_id = ?", organizationID, identityID)
	copilotThreads := db.NewSelect().TableExpr("customer_copilot_threads AS cct").
		ColumnExpr("cct.customer_conversation_id").
		Where("cct.organization_id = ?", organizationID).
		Where("cct.agent_identity_id = ? OR cct.created_by_identity_id = ?", identityID, identityID)
	runs := db.NewSelect().TableExpr("agent_runs AS agr").
		Column("agr.conversation_id").
		Where("agr.organization_id = ? AND agr.agent_identity_id = ?", organizationID, identityID)
	lanes := db.NewSelect().TableExpr("agent_lanes AS al").
		ColumnExpr("al.scope_id").
		Where("al.organization_id = ? AND al.agent_identity_id = ? AND al.scope_kind = ?", organizationID, identityID, domain.AgentExecutionScopeConversation)
	return touchProfileConversations(ctx, db, organizationID, participants.Union(assigned).Union(copilotThreads).Union(runs).Union(lanes), true)
}

// TouchChannelIdentityConversations 推进指定客户渠道身份所在客户会话的版本，访客页面不展示客户资料，不通知访客目录受众；调用方在完成该渠道身份的全部写入后调用。
func TouchChannelIdentityConversations(ctx context.Context, db bun.IDB, organizationID, channelIdentityID string) error {
	return touchProfileConversations(ctx, db, organizationID, db.NewSelect().TableExpr("customer_conversations AS cc").
		Column("cc.conversation_id").
		Where("cc.organization_id = ? AND cc.contact_channel_identity_id = ?", organizationID, channelIdentityID), false)
}

// TouchContactConversations 推进以联系人名称展示客户的会话版本，即渠道身份没有名称的客户会话，不通知访客目录受众；调用方在完成该联系人的全部写入后调用。
func TouchContactConversations(ctx context.Context, db bun.IDB, organizationID, contactID string) error {
	return touchProfileConversations(ctx, db, organizationID, db.NewSelect().TableExpr("customer_conversations AS cc").
		Column("cc.conversation_id").
		Join("JOIN contact_channel_identities AS cci ON cci.organization_id = cc.organization_id AND cci.id = cc.contact_channel_identity_id").
		Where("cc.organization_id = ? AND cci.contact_id = ? AND cci.display_name IS NULL", organizationID, contactID), false)
}

// TouchChannelConversations 推进指定渠道下全部客户会话的版本，访客页面不展示渠道名称，不通知访客目录受众；调用方在完成该渠道的全部写入后调用。
func TouchChannelConversations(ctx context.Context, db bun.IDB, organizationID, channelID string) error {
	return touchProfileConversations(ctx, db, organizationID, db.NewSelect().TableExpr("customer_conversations AS cc").
		Column("cc.conversation_id").
		Join("JOIN contact_channel_identities AS cci ON cci.organization_id = cc.organization_id AND cci.id = cc.contact_channel_identity_id").
		Where("cc.organization_id = ? AND cci.channel_id = ?", organizationID, channelID), false)
}

// touchProfileConversations 按会话 ID 顺序锁定资料展示所在的会话，推进版本并登记成员与客服受众通知；notifyVisitor 为真时同时登记网站访客目录受众。
func touchProfileConversations(ctx context.Context, db bun.IDB, organizationID string, conversationIDs *bun.SelectQuery, notifyVisitor bool) error {
	var conversations []*servermodels.Conversation
	if err := db.NewSelect().Model(&conversations).
		Column("cv.id").
		Where("cv.organization_id = ? AND cv.id IN (?)", organizationID, conversationIDs).
		OrderExpr("cv.id").
		For("UPDATE").
		Scan(ctx); err != nil {
		return fmt.Errorf("lock profile conversations: %w", err)
	}
	for _, conversation := range conversations {
		if err := db.NewUpdate().Model(conversation).
			Set("version = version + 1").
			WherePK().Where("organization_id = ?", organizationID).
			Returning("*").Scan(ctx); err != nil {
			return fmt.Errorf("advance profile conversation version: %w", err)
		}
		if err := notifyConversationChanged(ctx, db, conversation, notifyVisitor); err != nil {
			return err
		}
	}
	return nil
}
