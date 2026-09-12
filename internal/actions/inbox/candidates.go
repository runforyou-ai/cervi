//go:build server

package inbox

import (
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// customerConversationAccessQuery 共用客户阅读范围和公开摘要所需的有效关联。
func (q *LoadInboxQuery) customerConversationAccessQuery(organizationID string) *bun.SelectQuery {
	return q.db.NewSelect().TableExpr("customer_conversations AS cc").
		ColumnExpr("cv.id, cv.last_activity_at").
		Join("JOIN conversations AS cv ON cv.id = cc.conversation_id AND cv.organization_id = cc.organization_id").
		Join("JOIN contact_channel_identities AS cci ON cci.id = cc.contact_channel_identity_id AND cci.organization_id = cc.organization_id").
		Join("JOIN contacts AS c ON c.id = cci.contact_id AND c.organization_id = cc.organization_id").
		Join("JOIN channels AS ch ON ch.id = cci.channel_id AND ch.organization_id = cc.organization_id").
		Join("LEFT JOIN messages AS msg ON msg.id = cv.last_message_id AND msg.organization_id = cv.organization_id AND msg.conversation_id = cv.id AND msg.deleted_at IS NULL").
		Join("JOIN service_sessions AS current ON current.organization_id = cc.organization_id AND current.conversation_id = cc.conversation_id AND current.id = cc.current_service_session_id").
		Where("cc.organization_id = ?", organizationID).
		Where("cv.type = ?", domain.ConversationTypeCustomer)
}

// memberConversationAccessQuery 按当前有效成员关系读取活跃或归档的内部会话。
func (q *LoadInboxQuery) memberConversationAccessQuery(organizationID, identityID string) *bun.SelectQuery {
	return q.db.NewSelect().TableExpr("conversations AS cv").
		ColumnExpr("cv.id, cv.last_activity_at").
		Join("JOIN conversation_participants AS mine ON mine.organization_id = cv.organization_id AND mine.conversation_id = cv.id AND mine.left_at IS NULL").
		Join("JOIN chat_subjects AS mine_cs ON mine_cs.id = mine.subject_id AND mine_cs.organization_id = mine.organization_id AND mine_cs.kind = ? AND mine_cs.source_id = ?", domain.ChatSubjectKindOrganizationIdentity, identityID).
		Where("cv.organization_id = ?", organizationID).
		Where("cv.status IN (?, ?)", domain.ConversationStatusActive, domain.ConversationStatusArchived)
}

// directConversationAccessQuery 以真人身份对和有效成员关系限定阅读范围。
func (q *LoadInboxQuery) directConversationAccessQuery(organizationID, identityID string) *bun.SelectQuery {
	return q.memberConversationAccessQuery(organizationID, identityID).
		Join("JOIN direct_conversations AS dc ON dc.organization_id = cv.organization_id AND dc.conversation_id = cv.id").
		Join("JOIN organization_identities AS peer_oi ON peer_oi.organization_id = dc.organization_id AND peer_oi.id = CASE WHEN dc.first_identity_id = ? THEN dc.second_identity_id ELSE dc.first_identity_id END", identityID).
		Join("JOIN users AS peer_u ON peer_u.organization_id = peer_oi.organization_id AND peer_u.identity_id = peer_oi.id").
		Where("cv.type = ?", domain.ConversationTypeDirect).
		Where("? IN (dc.first_identity_id, dc.second_identity_id)", identityID).
		Where("peer_oi.type = ?", domain.OrganizationIdentityTypeUser)
}

// agentConversationAccessQuery 以固定业务归属和有效成员关系限定 AI 会话阅读范围。
func (q *LoadInboxQuery) agentConversationAccessQuery(organizationID, identityID string) *bun.SelectQuery {
	return q.memberConversationAccessQuery(organizationID, identityID).
		Join("JOIN agent_conversations AS ac ON ac.organization_id = cv.organization_id AND ac.conversation_id = cv.id").
		Join("JOIN organization_identities AS oi ON oi.organization_id = ac.organization_id AND oi.id = ac.agent_identity_id").
		Join("JOIN agents AS agent ON agent.organization_id = oi.organization_id AND agent.identity_id = oi.id").
		Where("cv.type = ? AND ac.user_identity_id = ? AND oi.type = ?", domain.ConversationTypeAgent, identityID, domain.OrganizationIdentityTypeAgent)
}

// groupConversationAccessQuery 限定当前成员可读的群聊，解散后仍保留阅读资格。
func (q *LoadInboxQuery) groupConversationAccessQuery(organizationID, identityID string) *bun.SelectQuery {
	return q.memberConversationAccessQuery(organizationID, identityID).Where("cv.type = ?", domain.ConversationTypeGroup)
}

// directConversationsQuery 限定当前列表中的活跃真人单聊。
func (q *LoadInboxQuery) directConversationsQuery(organizationID, identityID string) *bun.SelectQuery {
	return q.directConversationAccessQuery(organizationID, identityID).Where("cv.status = ? AND peer_u.status = ?", domain.ConversationStatusActive, domain.UserStatusActive)
}

// agentConversationsQuery 限定当前列表中的活跃 AI 聊天。
func (q *LoadInboxQuery) agentConversationsQuery(organizationID, identityID string) *bun.SelectQuery {
	return q.agentConversationAccessQuery(organizationID, identityID).Where("cv.status = ? AND agent.status = ?", domain.ConversationStatusActive, domain.UserStatusActive)
}

// listCandidates 共用列表分页与按 ID 资格判断的最小候选投影。
func (q *LoadInboxQuery) listCandidates(identity *servermodels.Identity, input LoadInput) *bun.SelectQuery {
	organizationID, identityID := identity.Organization.ID, identity.OrganizationIdentity.ID
	queries := make([]*bun.SelectQuery, 0, 4)
	if input.Scope != domain.InboxScopeInternal && input.includesKind(domain.ConversationTypeCustomer) {
		queries = append(queries, filterCustomerInbox(q.customerConversationAccessQuery(organizationID), identityID, input))
	}
	if input.Scope != domain.InboxScopeCustomer {
		if input.includesKind(domain.ConversationTypeDirect) {
			queries = append(queries, q.directConversationsQuery(organizationID, identityID))
		}
		if input.includesKind(domain.ConversationTypeAgent) {
			queries = append(queries, q.agentConversationsQuery(organizationID, identityID))
		}
		if input.includesKind(domain.ConversationTypeGroup) {
			queries = append(queries, q.groupConversationAccessQuery(organizationID, identityID))
		}
	}
	candidate := queries[0]
	for _, query := range queries[1:] {
		candidate = candidate.UnionAll(query)
	}
	return candidate
}
