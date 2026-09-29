//go:build server

package chatstate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/pkg/searchtext"
	"github.com/uptrace/bun"
)

// AppendMessage 在调用方事务和会话锁内追加消息、推进会话版本、维护摘要并登记会话受众通知；调用方负责授权及完整发送意图校验。
func AppendMessage(ctx context.Context, db bun.IDB, conversation *servermodels.Conversation, message *servermodels.Message) (*servermodels.Message, bool, error) {
	// 幂等重放返回既有消息并保留序号和摘要。
	if message.IdempotencyKey != nil {
		existing := &servermodels.Message{}
		err := db.NewSelect().Model(existing).
			Where("msg.organization_id = ? AND msg.conversation_id = ? AND msg.idempotency_key = ?", conversation.OrganizationID, conversation.ID, *message.IdempotencyKey).
			Scan(ctx)
		if err == nil {
			return existing, false, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, false, fmt.Errorf("load appended message: %w", err)
		}
	}
	// 序号与会话版本同句推进。
	if err := db.NewUpdate().Model(conversation).
		Set("last_message_seq = last_message_seq + 1").
		Set("version = version + 1").
		WherePK().Where("organization_id = ?", conversation.OrganizationID).
		Returning("last_message_seq, version").Scan(ctx); err != nil {
		return nil, false, fmt.Errorf("allocate message sequence: %w", err)
	}
	message.MessageSeq = conversation.LastMessageSeq
	if message.Visibility == "" {
		message.Visibility = string(domain.MessageVisibilityShared)
	}
	// 文本消息按正文生成检索词元；附件消息与翻译发送的文本由调用方生成。
	if message.Type == string(domain.MessageTypeText) && message.SearchVector == "" {
		message.SearchVector = searchtext.Vector(message.Body)
	}
	if _, err := db.NewInsert().Model(message).
		Column("id", "organization_id", "conversation_id", "service_session_id", "sender_participant_id", "type", "visibility", "body", "language", "search_vector", "system_event_type", "system_event_payload", "reply_to_message_id", "mention_all", "idempotency_key", "client_message_id", "originated_at", "source_order", "message_seq").
		Returning("*").Exec(ctx); err != nil {
		return nil, false, fmt.Errorf("append conversation message: %w", err)
	}
	// 周期摘要只记录共享消息；发起人的消息开始或延续等待回复，处理方的共享消息结束等待；等待起点变化时清空本轮提醒时间；新的共享消息清空 AI 请求确认解决的时间。
	if message.ServiceSessionID != nil && message.Visibility == string(domain.MessageVisibilityShared) {
		fromRequester := db.NewSelect().TableExpr("conversation_participants AS cp").ColumnExpr("1").
			Join("JOIN service_conversations AS svc ON svc.organization_id = cp.organization_id AND svc.conversation_id = cp.conversation_id AND svc.requester_subject_id = cp.subject_id").
			Where("cp.organization_id = ? AND cp.id = ?", conversation.OrganizationID, message.SenderParticipantID)
		if _, err := db.NewUpdate().Model((*servermodels.ServiceSession)(nil)).
			Set("last_message_id = ?", message.ID).
			Set("last_message_at = ?", message.OriginatedAt).
			Set("awaiting_reply_since = CASE WHEN EXISTS (?) THEN COALESCE(awaiting_reply_since, ?) ELSE NULL END", fromRequester, message.OriginatedAt).
			Set("reminded_at = CASE WHEN EXISTS (?) AND awaiting_reply_since IS NOT NULL THEN reminded_at ELSE NULL END", fromRequester).
			Set("resolution_requested_at = NULL").
			Set("updated_at = now()").
			Where("organization_id = ? AND conversation_id = ? AND id = ?", conversation.OrganizationID, conversation.ID, *message.ServiceSessionID).
			Where("status = ?", domain.ServiceSessionStatusOpen).
			Exec(ctx); err != nil {
			return nil, false, fmt.Errorf("update service session summary: %w", err)
		}
	}
	// 活动时间取锁内数据库时钟，保留同会话已提交的较大值；会话摘要与活动时间只随会话各方或发起人可见的消息推进，内部消息与服务周期的流转事件不改变，发给发起人的服务进度推进。
	if message.Visibility != string(domain.MessageVisibilityInternal) &&
		(message.Type != string(domain.MessageTypeSystem) || message.ServiceSessionID == nil || message.Visibility == string(domain.MessageVisibilityRequester)) {
		query := db.NewUpdate().Model(conversation).
			Set("last_activity_at = GREATEST(last_activity_at, clock_timestamp())").
			Set("last_message_id = ?", message.ID).
			Set("last_message_at = ?", message.OriginatedAt).
			Set("updated_at = now()").
			WherePK().Where("organization_id = ?", conversation.OrganizationID)
		if err := query.Returning("last_activity_at").Scan(ctx); err != nil {
			return nil, false, fmt.Errorf("update conversation summary: %w", err)
		}
	} else if message.Visibility == string(domain.MessageVisibilityInternal) && message.Type != string(domain.MessageTypeSystem) {
		// 内部备注与内部错误只推进内部活动时间，供处理方的服务列表排序。
		if err := db.NewUpdate().Model(conversation).
			Set("last_internal_activity_at = GREATEST(last_internal_activity_at, clock_timestamp())").
			Set("updated_at = now()").
			WherePK().Where("organization_id = ?", conversation.OrganizationID).
			Returning("last_internal_activity_at").Scan(ctx); err != nil {
			return nil, false, fmt.Errorf("update conversation internal activity: %w", err)
		}
	}
	// 内部备注不登记网站访客受众的变更通知；系统事件全部通知访客，访客据此拉取新事件并同步周期评价状态。
	notifyVisitor := message.Visibility != string(domain.MessageVisibilityInternal) || message.Type == string(domain.MessageTypeSystem)
	// 消息改变时间线，系统事件另按事件类型带上参与方或服务周期变化。
	changes := domain.ConversationChangeTimeline
	if message.SystemEventType != nil {
		changes |= domain.ConversationSystemEventType(*message.SystemEventType).ConversationChanges()
	}
	if err := notifyConversationChanged(ctx, db, conversation, changes, notifyVisitor); err != nil {
		return nil, false, err
	}
	return message, true, nil
}

// TouchConversation 在调用方持有会话锁的事务内推进会话版本，并登记带指定变化类别的会话受众变更通知。
func TouchConversation(ctx context.Context, db bun.IDB, conversation *servermodels.Conversation, changes domain.ConversationChanges) error {
	if err := db.NewUpdate().Model(conversation).
		Set("version = version + 1").
		WherePK().Where("organization_id = ?", conversation.OrganizationID).
		Returning("version").Scan(ctx); err != nil {
		return fmt.Errorf("advance conversation version: %w", err)
	}
	return notifyConversationChanged(ctx, db, conversation, changes, true)
}

// NotifyConversationChanged 按会话当前版本为成员受众补登记变化类别，不推进版本也不通知网站访客；同一事务内已登记的通知与之合并。
func NotifyConversationChanged(ctx context.Context, db bun.IDB, conversation *servermodels.Conversation, changes domain.ConversationChanges) error {
	return notifyConversationChanged(ctx, db, conversation, changes, false)
}

// notifyConversationChanged 按会话当前版本登记单个会话的变更通知，受众规则同 notifyConversationsChanged。
func notifyConversationChanged(ctx context.Context, db bun.IDB, conversation *servermodels.Conversation, changes domain.ConversationChanges, notifyVisitor bool) error {
	return notifyConversationsChanged(ctx, db, conversation.OrganizationID, []*servermodels.Conversation{conversation}, changes, notifyVisitor)
}

// notifyConversationsChanged 按会话当前版本批量登记变更通知：客户会话及其 Copilot 线程通知企业客服共享受众，网站客户会话在 notifyVisitor 为真时同时通知所属渠道身份受众，内部会话通知当前真人成员，承载服务会话的 AI 聊天另外通知企业客服共享受众；各类受众按会话批量查询。
func notifyConversationsChanged(ctx context.Context, db bun.IDB, organizationID string, conversations []*servermodels.Conversation, changes domain.ConversationChanges, notifyVisitor bool) error {
	byID := make(map[string]*servermodels.Conversation, len(conversations))
	var customerIDs, agentIDs, memberIDs []string
	for _, conversation := range conversations {
		byID[conversation.ID] = conversation
		switch conversationType := domain.ConversationType(conversation.Type); conversationType {
		case domain.ConversationTypeChannel, domain.ConversationTypeCopilot:
			realtime.Notify(ctx, realtime.ServiceInboxConversationChanged(organizationID, conversation.ID, conversationType, conversation.Version, changes))
			if conversationType == domain.ConversationTypeChannel && notifyVisitor {
				customerIDs = append(customerIDs, conversation.ID)
			}
		case domain.ConversationTypeAgent:
			agentIDs = append(agentIDs, conversation.ID)
			memberIDs = append(memberIDs, conversation.ID)
		default:
			memberIDs = append(memberIDs, conversation.ID)
		}
	}
	// 仅网站客户会话按所属渠道身份登记访客目录受众通知。
	if len(customerIDs) > 0 {
		var visitors []struct {
			ConversationID    string `bun:"conversation_id"`
			ChannelIdentityID string `bun:"channel_identity_id"`
		}
		if err := db.NewSelect().TableExpr("channel_conversations AS cc").
			ColumnExpr("cc.conversation_id, cci.id AS channel_identity_id").
			Join("JOIN contact_channel_identities AS cci ON cci.organization_id = cc.organization_id AND cci.id = cc.contact_channel_identity_id").
			Join("JOIN channels AS c ON c.organization_id = cci.organization_id AND c.id = cci.channel_id AND c.type = ?", domain.ChannelTypeWebsite).
			Where("cc.organization_id = ? AND cc.conversation_id IN (?)", organizationID, bun.In(customerIDs)).
			Scan(ctx, &visitors); err != nil {
			return fmt.Errorf("load visitor notification audience: %w", err)
		}
		for _, visitor := range visitors {
			realtime.Notify(ctx, realtime.VisitorDirectoryConversationChanged(organizationID, visitor.ChannelIdentityID, visitor.ConversationID, byID[visitor.ConversationID].Version))
		}
	}
	// AI 聊天承载服务会话时同时通知企业客服共享受众。
	if len(agentIDs) > 0 {
		var served []string
		if err := db.NewSelect().Model((*servermodels.ServiceConversation)(nil)).Column("svc.conversation_id").
			Where("svc.organization_id = ? AND svc.conversation_id IN (?)", organizationID, bun.In(agentIDs)).
			Scan(ctx, &served); err != nil {
			return fmt.Errorf("check service conversation notification audience: %w", err)
		}
		for _, conversationID := range served {
			realtime.Notify(ctx, realtime.ServiceInboxConversationChanged(organizationID, conversationID, domain.ConversationTypeAgent, byID[conversationID].Version, changes))
		}
	}
	if len(memberIDs) > 0 {
		var members []struct {
			ConversationID string `bun:"conversation_id"`
			UserID         string `bun:"user_id"`
		}
		if err := db.NewSelect().TableExpr("conversation_participants AS cp").
			ColumnExpr("cp.conversation_id, u.id AS user_id").
			Join("JOIN chat_subjects AS cs ON cs.organization_id = cp.organization_id AND cs.id = cp.subject_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
			Join("JOIN users AS u ON u.organization_id = cs.organization_id AND u.identity_id = cs.source_id").
			Where("cp.organization_id = ? AND cp.conversation_id IN (?) AND cp.left_at IS NULL", organizationID, bun.In(memberIDs)).
			Scan(ctx, &members); err != nil {
			return fmt.Errorf("load conversation notification audience: %w", err)
		}
		for _, member := range members {
			conversation := byID[member.ConversationID]
			realtime.Notify(ctx, realtime.UserConversationChanged(organizationID, member.UserID, conversation.ID, domain.ConversationType(conversation.Type), conversation.Version, changes))
		}
	}
	return nil
}
