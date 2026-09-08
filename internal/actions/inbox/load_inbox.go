//go:build server

// Package inbox 实现统一收件箱领域的应用查询。
package inbox

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/storage/server/messagequery"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const defaultInboxPageSize = 50

// LoadInput 定义统一收件箱筛选、页大小和分页边界。
type LoadInput struct {
	Cursor             string
	Limit              int
	Scope              domain.InboxScope
	CustomerView       domain.CustomerInboxView
	AssigneeIdentityID string
}

// AssigneeSummary 定义客户会话负责人摘要。
type AssigneeSummary struct {
	IdentityID   string
	Type         domain.OrganizationIdentityType
	DisplayName  string
	AvatarFileID *string
}

// CustomerConversationSummary 定义收件箱中的客户会话详情。
type CustomerConversationSummary struct {
	Title                     string
	ContactName               *string
	ContactAvatarFileID       *string
	ChannelType               domain.ChannelType
	ChannelName               string
	Preview                   *string
	PreviewSenderIdentityType *domain.OrganizationIdentityType
	LastMessageAt             *time.Time
	ServiceSessionStatus      domain.ServiceSessionStatus
	ServiceSessionID          string
	Assignee                  *AssigneeSummary
}

// DirectConversationSummary 定义收件箱中的内部单聊详情。
type DirectConversationSummary struct {
	PeerIdentityID            string
	PeerType                  domain.OrganizationIdentityType
	PeerName                  string
	PeerAvatarFileID          *string
	Preview                   *string
	PreviewSenderIdentityType *domain.OrganizationIdentityType
	LastMessageAt             *time.Time
}

// AgentConversationSummary 定义收件箱中的 AI 聊天详情。
type AgentConversationSummary struct {
	Title                     string
	AgentIdentityID           string
	AgentName                 string
	AgentAvatarFileID         *string
	Preview                   *string
	PreviewSenderIdentityType *domain.OrganizationIdentityType
	LastMessageAt             *time.Time
	AgentRunStatus            *domain.AgentRunStatus
}

// GroupConversationSummary 定义收件箱中的企业群聊详情。
type GroupConversationSummary struct {
	Title                     string
	ImageFileID               *string
	Status                    domain.ConversationStatus
	Preview                   *string
	PreviewSenderIdentityType *domain.OrganizationIdentityType
	LastMessageAt             *time.Time
	MemberCount               int
}

// ConversationSummary 定义统一收件箱会话信封。
type ConversationSummary struct {
	LastActivityAt       *time.Time
	LastMessageType      *domain.MessageType
	ID                   string
	Type                 domain.ConversationType
	UnreadCount          int
	MentionedUnreadCount int
	Muted                bool
	MarkedUnread         bool
	LastMessageID        *string
	LastReadMessageID    *string
	Customer             *CustomerConversationSummary
	Agent                *AgentConversationSummary
	Direct               *DirectConversationSummary
	Group                *GroupConversationSummary
}

// LoadInboxQuery 读取当前企业的统一收件箱。
type LoadInboxQuery struct {
	db bun.IDB
}

// UnreadCounts 定义内部会话的客观未读和提醒未读总数。
type UnreadCounts struct {
	Unread    int `bun:"unread_count"`
	Attention int `bun:"attention_unread_count"`
}

type customerConversationRow struct {
	ID                        string                           `bun:"id"`
	Title                     string                           `bun:"title"`
	ContactName               *string                          `bun:"contact_name"`
	ContactAvatarFileID       *string                          `bun:"contact_avatar_file_id"`
	ChannelType               string                           `bun:"channel_type"`
	ChannelName               string                           `bun:"channel_name"`
	Preview                   *string                          `bun:"preview"`
	PreviewSenderIdentityType *domain.OrganizationIdentityType `bun:"preview_sender_identity_type"`
	LastMessageAt             *time.Time                       `bun:"last_message_at"`
	ServiceSessionStatus      string                           `bun:"service_session_status"`
	ServiceSessionID          string                           `bun:"service_session_id"`
	AssigneeIdentityID        *string                          `bun:"assignee_identity_id"`
	AssigneeType              *string                          `bun:"assignee_type"`
	AssigneeDisplayName       *string                          `bun:"assignee_display_name"`
	AssigneeAvatarFileID      *string                          `bun:"assignee_avatar_file_id"`
	LastActivityAt            *time.Time                       `bun:"last_activity_at"`
	UnreadCount               int                              `bun:"unread_count"`
	LastReadMessageID         *string                          `bun:"last_read_message_id"`
	LastMessageID             *string                          `bun:"last_message_id"`
	LastMessageType           *domain.MessageType              `bun:"last_message_type"`
}

type directConversationRow struct {
	ID                        string                           `bun:"id"`
	PeerIdentityID            string                           `bun:"peer_identity_id"`
	PeerType                  string                           `bun:"peer_type"`
	PeerName                  string                           `bun:"peer_name"`
	PeerAvatarFileID          *string                          `bun:"peer_avatar_file_id"`
	Preview                   *string                          `bun:"preview"`
	PreviewSenderIdentityType *domain.OrganizationIdentityType `bun:"preview_sender_identity_type"`
	LastMessageAt             *time.Time                       `bun:"last_message_at"`
	LastActivityAt            *time.Time                       `bun:"last_activity_at"`
	UnreadCount               int                              `bun:"unread_count"`
	LastMessageID             *string                          `bun:"last_message_id"`
	LastMessageType           *domain.MessageType              `bun:"last_message_type"`
	LastReadMessageID         *string                          `bun:"last_read_message_id"`
	Muted                     bool                             `bun:"muted"`
	MarkedUnread              bool                             `bun:"marked_unread"`
}

type agentConversationRow struct {
	Title                     string                           `bun:"title"`
	ID                        string                           `bun:"id"`
	AgentIdentityID           string                           `bun:"agent_identity_id"`
	AgentName                 string                           `bun:"agent_name"`
	AgentAvatarFileID         *string                          `bun:"agent_avatar_file_id"`
	Preview                   *string                          `bun:"preview"`
	PreviewSenderIdentityType *domain.OrganizationIdentityType `bun:"preview_sender_identity_type"`
	LastMessageAt             *time.Time                       `bun:"last_message_at"`
	AgentRunStatus            *string                          `bun:"agent_run_status"`
	LastActivityAt            *time.Time                       `bun:"last_activity_at"`
	UnreadCount               int                              `bun:"unread_count"`
	LastMessageID             *string                          `bun:"last_message_id"`
	LastMessageType           *domain.MessageType              `bun:"last_message_type"`
	LastReadMessageID         *string                          `bun:"last_read_message_id"`
	Muted                     bool                             `bun:"muted"`
	MarkedUnread              bool                             `bun:"marked_unread"`
}

type groupConversationRow struct {
	ID                        string                           `bun:"id"`
	Title                     string                           `bun:"title"`
	ImageFileID               *string                          `bun:"image_file_id"`
	Status                    string                           `bun:"status"`
	Preview                   *string                          `bun:"preview"`
	PreviewSenderIdentityType *domain.OrganizationIdentityType `bun:"preview_sender_identity_type"`
	LastMessageAt             *time.Time                       `bun:"last_message_at"`
	MemberCount               int                              `bun:"member_count"`
	LastActivityAt            *time.Time                       `bun:"last_activity_at"`
	UnreadCount               int                              `bun:"unread_count"`
	MentionedUnreadCount      int                              `bun:"mentioned_unread_count"`
	LastMessageID             *string                          `bun:"last_message_id"`
	LastMessageType           *domain.MessageType              `bun:"last_message_type"`
	LastReadMessageID         *string                          `bun:"last_read_message_id"`
	Muted                     bool                             `bun:"muted"`
	MarkedUnread              bool                             `bun:"marked_unread"`
}

// NewLoadInboxQuery 创建成员收件箱查询。
func NewLoadInboxQuery(db bun.IDB) *LoadInboxQuery {
	return &LoadInboxQuery{db: db}
}

// ConversationPage 保存统一排序的一页会话及后续边界。
type ConversationPage struct {
	Conversations []ConversationSummary
	NextCursor    string
	HasMore       bool
}

// Execute 在同一只读快照中读取会话分页与完整未读总数。
func (q *LoadInboxQuery) Execute(ctx context.Context, identity *servermodels.Identity, input LoadInput) (ConversationPage, UnreadCounts, error) {
	input, err := normalizeLoadInput(input)
	if err != nil {
		return ConversationPage{}, UnreadCounts{}, err
	}
	// 为 limit + 1 的续页探测保留一个整数位置。
	if input.Limit < 0 || input.Limit == math.MaxInt {
		return ConversationPage{}, UnreadCounts{}, ErrQueryInvalid
	}
	if input.Limit == 0 {
		input.Limit = defaultInboxPageSize
	}
	var boundary *inboxCursor
	if input.Cursor != "" {
		boundary, err = decodeInboxCursor(input.Cursor, identity, input)
		if err != nil {
			return ConversationPage{}, UnreadCounts{}, err
		}
	}
	var page ConversationPage
	var counts UnreadCounts
	err = q.db.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		snapshot := NewLoadInboxQuery(tx)
		var err error
		page, err = snapshot.loadConversationPage(ctx, identity, input, boundary)
		if err != nil {
			return err
		}
		counts, err = snapshot.loadUnreadCounts(ctx, identity.Organization.ID, identity.OrganizationIdentity.ID, identity.User.ID)
		return err
	})
	return page, counts, err
}

// loadConversationPage 按精确活动边界取整页，边界行移除不影响后续读取。
func (q *LoadInboxQuery) loadConversationPage(ctx context.Context, identity *servermodels.Identity, input LoadInput, boundary *inboxCursor) (ConversationPage, error) {
	query := q.db.NewSelect().TableExpr("(?) AS candidates", q.listCandidates(identity, input)).ColumnExpr("id, last_activity_at")
	if boundary != nil {
		if boundary.LastActivityAt == nil {
			query.Where("last_activity_at IS NULL AND id < ?", boundary.ID)
		} else {
			query.Where("(last_activity_at, id) < (?, ?) OR last_activity_at IS NULL", *boundary.LastActivityAt, boundary.ID)
		}
	}
	var points []inboxCursorPoint
	if err := query.OrderExpr("last_activity_at DESC NULLS LAST, id DESC").Limit(input.Limit+1).Scan(ctx, &points); err != nil {
		return ConversationPage{}, fmt.Errorf("list inbox page: %w", err)
	}
	page := ConversationPage{Conversations: make([]ConversationSummary, 0, min(len(points), input.Limit)), HasMore: len(points) > input.Limit}
	if page.HasMore {
		points = points[:input.Limit]
		var err error
		page.NextCursor, err = encodeInboxCursor(identity, input, points[len(points)-1])
		if err != nil {
			return ConversationPage{}, err
		}
	}
	if len(points) == 0 {
		return page, nil
	}
	ids := make([]string, len(points))
	for index, point := range points {
		ids[index] = point.ID
	}
	summaries, err := q.readSummaries(ctx, identity, ids)
	if err != nil {
		return ConversationPage{}, err
	}
	// 候选与摘要共用阅读资格并在同一快照内读取，每个编号都有对应摘要。
	for _, id := range ids {
		page.Conversations = append(page.Conversations, *summaries[id])
	}
	return page, nil
}

// customerConversationDetailsQuery 读取企业内客户会话摘要，不按处理队列限制阅读。
func (q *LoadInboxQuery) customerConversationDetailsQuery(organizationID, currentIdentityID, userID string) *bun.SelectQuery {
	return q.customerConversationAccessQuery(organizationID).
		ColumnExpr("unread.unread_count AS unread_count").
		ColumnExpr("state.last_read_message_id::text AS last_read_message_id").
		ColumnExpr("cv.title AS title").
		ColumnExpr("COALESCE(cci.display_name, c.display_name) AS contact_name").
		ColumnExpr("cci.avatar_file_id AS contact_avatar_file_id").
		ColumnExpr("ch.type AS channel_type").
		ColumnExpr("ch.name AS channel_name").
		ColumnExpr("? AS preview", messagequery.Summary("msg")).
		ColumnExpr("msg.type AS last_message_type").
		ColumnExpr("preview_oi.type AS preview_sender_identity_type").
		ColumnExpr("cv.last_message_at AS last_message_at").
		ColumnExpr("cv.last_message_id::text AS last_message_id").
		ColumnExpr("current.status AS service_session_status").
		ColumnExpr("current.id::text AS service_session_id").
		ColumnExpr("current.assignee_identity_id::text AS assignee_identity_id").
		ColumnExpr("assignee.type AS assignee_type").
		ColumnExpr("assignee.display_name AS assignee_display_name").
		ColumnExpr("assignee.avatar_file_id::text AS assignee_avatar_file_id").
		Join("LEFT JOIN conversation_participants AS preview_cp ON preview_cp.id = msg.sender_participant_id AND preview_cp.organization_id = msg.organization_id AND preview_cp.conversation_id = msg.conversation_id").
		Join("LEFT JOIN chat_subjects AS preview_cs ON preview_cs.id = preview_cp.subject_id AND preview_cs.organization_id = preview_cp.organization_id").
		Join("LEFT JOIN organization_identities AS preview_oi ON preview_oi.id = preview_cs.source_id AND preview_oi.organization_id = preview_cs.organization_id AND preview_cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("LEFT JOIN organization_identities AS assignee ON assignee.organization_id = cv.organization_id AND assignee.id = current.assignee_identity_id").
		Join("LEFT JOIN conversation_user_states AS state ON state.organization_id = cv.organization_id AND state.conversation_id = cv.id AND state.user_id = ?", userID).
		Join(`JOIN LATERAL (
			SELECT count(*) AS unread_count
			FROM messages AS unread_msg
			JOIN conversation_participants AS sender_cp ON sender_cp.organization_id = unread_msg.organization_id AND sender_cp.conversation_id = unread_msg.conversation_id AND sender_cp.id = unread_msg.sender_participant_id
			JOIN chat_subjects AS sender_cs ON sender_cs.organization_id = sender_cp.organization_id AND sender_cs.id = sender_cp.subject_id
			WHERE unread_msg.organization_id = cv.organization_id AND unread_msg.conversation_id = cv.id
				AND unread_msg.type IN (?) AND unread_msg.deleted_at IS NULL
				AND NOT (sender_cs.kind = ? AND sender_cs.source_id = ?)
				AND unread_msg.message_seq > COALESCE(state.read_seq, 0)
		) AS unread ON TRUE`, bun.In([]domain.MessageType{domain.MessageTypeText, domain.MessageTypeAgentError}), domain.ChatSubjectKindOrganizationIdentity, currentIdentityID)
}

// filterCustomerInbox 为客户摘要追加当前列表的筛选条件。
func filterCustomerInbox(query *bun.SelectQuery, currentIdentityID string, input LoadInput) *bun.SelectQuery {
	query = query.Where("msg.id IS NOT NULL")
	if input.Scope == domain.InboxScopeAll {
		query = query.
			Where("current.status = ?", domain.ServiceSessionStatusOpen).
			Where(`(
				current.assignee_identity_id = ?
				OR EXISTS (
					SELECT 1
					FROM conversation_participants AS related_cp
					JOIN chat_subjects AS related_cs
						ON related_cs.organization_id = related_cp.organization_id
						AND related_cs.id = related_cp.subject_id
					WHERE related_cp.organization_id = cv.organization_id
						AND related_cp.conversation_id = cv.id
						AND related_cs.kind = ?
						AND related_cs.source_id = ?
				)
			)`, currentIdentityID, domain.ChatSubjectKindOrganizationIdentity, currentIdentityID)
	} else {
		switch input.CustomerView {
		case domain.CustomerInboxViewQueue:
			query = query.Where("current.status = ?", domain.ServiceSessionStatusOpen).Where("current.assignee_identity_id IS NULL")
		case domain.CustomerInboxViewMine:
			query = query.Where("current.status = ?", domain.ServiceSessionStatusOpen).Where("current.assignee_identity_id = ?", currentIdentityID)
		case domain.CustomerInboxViewCoworkers:
			query = query.Where("current.status = ?", domain.ServiceSessionStatusOpen).
				Where("current.assignee_identity_id IS NOT NULL").
				Where("current.assignee_identity_id <> ?", currentIdentityID)
			if input.AssigneeIdentityID != "" {
				query = query.Where("current.assignee_identity_id = ?", input.AssigneeIdentityID)
			}
		case domain.CustomerInboxViewClosed:
			query = query.Where("current.status = ?", domain.ServiceSessionStatusClosed)
		}
	}
	return query
}

// withIndividualConversationDetails 为单聊阅读基线追加消息预览和个人未读状态。
func withIndividualConversationDetails(query *bun.SelectQuery, identityID, userID string) *bun.SelectQuery {
	return query.
		ColumnExpr("? AS preview", messagequery.Summary("msg")).
		ColumnExpr("msg.type AS last_message_type").
		ColumnExpr("preview_oi.type AS preview_sender_identity_type").
		ColumnExpr("cv.last_message_at AS last_message_at").
		ColumnExpr("cv.last_message_id::text AS last_message_id").
		ColumnExpr("unread.unread_count AS unread_count").
		ColumnExpr("state.last_read_message_id::text AS last_read_message_id").
		ColumnExpr("COALESCE(state.muted, false) AS muted").
		ColumnExpr("COALESCE(state.marked_unread, false) AS marked_unread").
		Join("LEFT JOIN messages AS msg ON msg.organization_id = cv.organization_id AND msg.conversation_id = cv.id AND msg.id = cv.last_message_id AND msg.deleted_at IS NULL").
		Join("LEFT JOIN conversation_participants AS preview_cp ON preview_cp.id = msg.sender_participant_id AND preview_cp.organization_id = msg.organization_id AND preview_cp.conversation_id = msg.conversation_id").
		Join("LEFT JOIN chat_subjects AS preview_cs ON preview_cs.id = preview_cp.subject_id AND preview_cs.organization_id = preview_cp.organization_id").
		Join("LEFT JOIN organization_identities AS preview_oi ON preview_oi.id = preview_cs.source_id AND preview_oi.organization_id = preview_cs.organization_id AND preview_cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("LEFT JOIN conversation_user_states AS state ON state.organization_id = cv.organization_id AND state.conversation_id = cv.id AND state.user_id = ?", userID).
		Join(`JOIN LATERAL (
			SELECT count(*) AS unread_count
			FROM messages AS unread_msg
			LEFT JOIN conversation_participants AS sender_cp ON sender_cp.organization_id = unread_msg.organization_id AND sender_cp.id = unread_msg.sender_participant_id
			LEFT JOIN chat_subjects AS sender_cs ON sender_cs.organization_id = sender_cp.organization_id AND sender_cs.id = sender_cp.subject_id
			WHERE unread_msg.organization_id = cv.organization_id AND unread_msg.conversation_id = cv.id AND unread_msg.deleted_at IS NULL
				AND (unread_msg.sender_participant_id IS NULL OR sender_cs.source_id <> ?)
				AND unread_msg.message_seq > COALESCE(state.read_seq, 0)
		) AS unread ON TRUE`, identityID)
}

// directConversationDetailsQuery 按真人身份对及有效成员关系读取长期单聊。
func (q *LoadInboxQuery) directConversationDetailsQuery(organizationID, identityID, userID string) *bun.SelectQuery {
	return withIndividualConversationDetails(q.directConversationAccessQuery(organizationID, identityID), identityID, userID).
		ColumnExpr("peer_oi.id AS peer_identity_id, peer_oi.type AS peer_type, peer_oi.display_name AS peer_name, peer_oi.avatar_file_id AS peer_avatar_file_id")
}

// agentConversationDetailsQuery 按业务归属和有效成员关系读取独立 AI 聊天。
func (q *LoadInboxQuery) agentConversationDetailsQuery(organizationID, identityID, userID string) *bun.SelectQuery {
	return withAgentConversationDetails(q.agentConversationAccessQuery(organizationID, identityID), identityID, userID)
}

// withAgentConversationDetails 为 AI 会话阅读基线追加消息摘要及当前运行状态。
func withAgentConversationDetails(query *bun.SelectQuery, identityID, userID string) *bun.SelectQuery {
	return withIndividualConversationDetails(query, identityID, userID).
		ColumnExpr("cv.title, oi.id AS agent_identity_id, oi.display_name AS agent_name, oi.avatar_file_id AS agent_avatar_file_id, latest_agent_run.status AS agent_run_status").
		Join("LEFT JOIN LATERAL (SELECT agr.status FROM agent_runs AS agr WHERE agr.organization_id = cv.organization_id AND agr.conversation_id = cv.id AND agr.agent_identity_id = ac.agent_identity_id ORDER BY agr.created_at DESC, agr.id DESC LIMIT 1) AS latest_agent_run ON TRUE")
}

// groupConversationsQuery 共用群聊成员范围、个人状态和未读统计。
func (q *LoadInboxQuery) groupConversationsQuery(organizationID, identityID, userID string) *bun.SelectQuery {
	return q.groupConversationAccessQuery(organizationID, identityID).
		ColumnExpr("cv.title AS title").
		ColumnExpr("cv.image_file_id::text AS image_file_id").
		ColumnExpr("cv.status AS status").
		ColumnExpr("? AS preview", messagequery.Summary("msg")).
		ColumnExpr("msg.type AS last_message_type").
		ColumnExpr("preview_oi.type AS preview_sender_identity_type").
		ColumnExpr("cv.last_message_at AS last_message_at").
		ColumnExpr("cv.last_message_id::text AS last_message_id").
		ColumnExpr("members.member_count AS member_count").
		ColumnExpr("unread.unread_count AS unread_count").
		ColumnExpr("unread.mentioned_unread_count AS mentioned_unread_count").
		ColumnExpr("state.last_read_message_id::text AS last_read_message_id").
		ColumnExpr("COALESCE(state.muted, false) AS muted").
		ColumnExpr("COALESCE(state.marked_unread, false) AS marked_unread").
		Join("JOIN LATERAL (SELECT count(*) AS member_count FROM conversation_participants AS member_cp WHERE member_cp.organization_id = cv.organization_id AND member_cp.conversation_id = cv.id AND member_cp.left_at IS NULL) AS members ON TRUE").
		Join("LEFT JOIN messages AS msg ON msg.organization_id = cv.organization_id AND msg.conversation_id = cv.id AND msg.id = cv.last_message_id AND msg.deleted_at IS NULL").
		Join("LEFT JOIN conversation_participants AS preview_cp ON preview_cp.id = msg.sender_participant_id AND preview_cp.organization_id = msg.organization_id AND preview_cp.conversation_id = msg.conversation_id").
		Join("LEFT JOIN chat_subjects AS preview_cs ON preview_cs.id = preview_cp.subject_id AND preview_cs.organization_id = preview_cp.organization_id").
		Join("LEFT JOIN organization_identities AS preview_oi ON preview_oi.id = preview_cs.source_id AND preview_oi.organization_id = preview_cs.organization_id AND preview_cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("LEFT JOIN conversation_user_states AS state ON state.organization_id = cv.organization_id AND state.conversation_id = cv.id AND state.user_id = ?", userID).
		Join(`JOIN LATERAL (
			SELECT count(*) AS unread_count,
				count(*) FILTER (WHERE mention.message_id IS NOT NULL OR unread_msg.mention_all) AS mentioned_unread_count
			FROM messages AS unread_msg
			LEFT JOIN conversation_participants AS sender_cp ON sender_cp.organization_id = unread_msg.organization_id AND sender_cp.id = unread_msg.sender_participant_id
			LEFT JOIN chat_subjects AS sender_cs ON sender_cs.organization_id = sender_cp.organization_id AND sender_cs.id = sender_cp.subject_id
			LEFT JOIN message_mentions AS mention ON mention.organization_id = unread_msg.organization_id AND mention.message_id = unread_msg.id AND mention.subject_id = mine.subject_id
			WHERE unread_msg.organization_id = cv.organization_id AND unread_msg.conversation_id = cv.id AND unread_msg.deleted_at IS NULL
				AND (unread_msg.sender_participant_id IS NULL OR sender_cs.source_id <> ?)
				AND unread_msg.message_seq > COALESCE(state.read_seq, 0)
		) AS unread ON TRUE`, identityID)
}

// loadUnreadCounts 按完整会话范围汇总提醒，不受当前筛选和列表条数限制。
func (q *LoadInboxQuery) loadUnreadCounts(ctx context.Context, organizationID, identityID, userID string) (UnreadCounts, error) {
	direct := q.db.NewSelect().TableExpr("(?) AS direct", withIndividualConversationDetails(q.directConversationsQuery(organizationID, identityID), identityID, userID)).
		ColumnExpr("unread_count, 0::bigint AS mentioned_unread_count, muted, marked_unread")
	group := q.db.NewSelect().TableExpr("(?) AS groups", q.groupConversationsQuery(organizationID, identityID, userID)).
		ColumnExpr("unread_count, mentioned_unread_count, muted, marked_unread")
	agent := q.db.NewSelect().TableExpr("(?) AS agents", withIndividualConversationDetails(q.agentConversationsQuery(organizationID, identityID), identityID, userID)).ColumnExpr("unread_count, 0::bigint AS mentioned_unread_count, muted, marked_unread")
	counts := UnreadCounts{}
	err := q.db.NewSelect().TableExpr("(?) AS internal", direct.UnionAll(group).UnionAll(agent)).
		ColumnExpr("COALESCE(sum(unread_count), 0) AS unread_count").
		ColumnExpr(`COALESCE(sum(CASE WHEN muted THEN mentioned_unread_count
            ELSE GREATEST(unread_count, CASE WHEN marked_unread THEN 1 ELSE 0 END)
        END), 0) AS attention_unread_count`).Scan(ctx, &counts)
	if err != nil {
		return UnreadCounts{}, fmt.Errorf("count internal unread messages: %w", err)
	}
	return counts, nil
}

// summary 将 AI 会话查询结果转换为统一摘要。
func (row agentConversationRow) summary() ConversationSummary {
	var agentRunStatus *domain.AgentRunStatus
	if row.AgentRunStatus != nil {
		status := domain.AgentRunStatus(*row.AgentRunStatus)
		agentRunStatus = &status
	}
	return ConversationSummary{
		ID: row.ID, Type: domain.ConversationTypeAgent, UnreadCount: row.UnreadCount, Muted: row.Muted, MarkedUnread: row.MarkedUnread, LastMessageID: row.LastMessageID, LastMessageType: row.LastMessageType, LastReadMessageID: row.LastReadMessageID, LastActivityAt: row.LastActivityAt,
		Agent: &AgentConversationSummary{
			Title: row.Title, AgentIdentityID: row.AgentIdentityID, AgentName: row.AgentName, AgentAvatarFileID: row.AgentAvatarFileID,
			Preview: row.Preview, PreviewSenderIdentityType: row.PreviewSenderIdentityType, LastMessageAt: row.LastMessageAt, AgentRunStatus: agentRunStatus,
		},
	}
}

// LoadAgentConversation 读取当前成员指定 AI 会话的完整收件箱摘要。
func (q *LoadInboxQuery) LoadAgentConversation(ctx context.Context, identity *servermodels.Identity, conversationID string) (ConversationSummary, error) {
	var row agentConversationRow
	err := withAgentConversationDetails(q.agentConversationsQuery(identity.Organization.ID, identity.OrganizationIdentity.ID), identity.OrganizationIdentity.ID, identity.User.ID).Where("cv.id = ?", conversationID).Scan(ctx, &row)
	if err != nil {
		return ConversationSummary{}, err
	}
	return row.summary(), nil
}

// summary 转换客户会话的统一摘要。
func (row customerConversationRow) summary() ConversationSummary {
	var assignee *AssigneeSummary
	if row.AssigneeIdentityID != nil && row.AssigneeType != nil && row.AssigneeDisplayName != nil {
		assignee = &AssigneeSummary{IdentityID: *row.AssigneeIdentityID, Type: domain.OrganizationIdentityType(*row.AssigneeType), DisplayName: *row.AssigneeDisplayName, AvatarFileID: row.AssigneeAvatarFileID}
	}
	return ConversationSummary{
		ID: row.ID, Type: domain.ConversationTypeCustomer, UnreadCount: row.UnreadCount, LastMessageID: row.LastMessageID, LastMessageType: row.LastMessageType, LastReadMessageID: row.LastReadMessageID, LastActivityAt: row.LastActivityAt,
		Customer: &CustomerConversationSummary{
			Title: row.Title, ContactName: row.ContactName, ContactAvatarFileID: row.ContactAvatarFileID,
			ChannelType: domain.ChannelType(row.ChannelType), ChannelName: row.ChannelName,
			Preview: row.Preview, PreviewSenderIdentityType: row.PreviewSenderIdentityType, LastMessageAt: row.LastMessageAt,
			ServiceSessionID: row.ServiceSessionID, ServiceSessionStatus: domain.ServiceSessionStatus(row.ServiceSessionStatus), Assignee: assignee,
		},
	}
}

// summary 转换真人单聊会话的统一摘要。
func (row directConversationRow) summary() ConversationSummary {
	return ConversationSummary{
		ID: row.ID, Type: domain.ConversationTypeDirect, UnreadCount: row.UnreadCount, Muted: row.Muted, MarkedUnread: row.MarkedUnread, LastMessageID: row.LastMessageID, LastMessageType: row.LastMessageType, LastReadMessageID: row.LastReadMessageID, LastActivityAt: row.LastActivityAt,
		Direct: &DirectConversationSummary{
			PeerIdentityID: row.PeerIdentityID, PeerType: domain.OrganizationIdentityType(row.PeerType), PeerName: row.PeerName, PeerAvatarFileID: row.PeerAvatarFileID,
			Preview: row.Preview, PreviewSenderIdentityType: row.PreviewSenderIdentityType, LastMessageAt: row.LastMessageAt,
		},
	}
}

// summary 转换群聊会话的统一摘要。
func (row groupConversationRow) summary() ConversationSummary {
	return ConversationSummary{
		ID: row.ID, Type: domain.ConversationTypeGroup, UnreadCount: row.UnreadCount, MentionedUnreadCount: row.MentionedUnreadCount, Muted: row.Muted, MarkedUnread: row.MarkedUnread, LastMessageID: row.LastMessageID, LastMessageType: row.LastMessageType, LastReadMessageID: row.LastReadMessageID, LastActivityAt: row.LastActivityAt,
		Group: &GroupConversationSummary{
			Title: row.Title, ImageFileID: row.ImageFileID, Status: domain.ConversationStatus(row.Status), Preview: row.Preview, PreviewSenderIdentityType: row.PreviewSenderIdentityType,
			LastMessageAt: row.LastMessageAt, MemberCount: row.MemberCount,
		},
	}
}

// normalizeLoadInput 规范化并校验收件箱筛选。
func normalizeLoadInput(input LoadInput) (LoadInput, error) {
	input.Scope = domain.InboxScope(strings.TrimSpace(string(input.Scope)))
	input.CustomerView = domain.CustomerInboxView(strings.TrimSpace(string(input.CustomerView)))
	input.AssigneeIdentityID = strings.TrimSpace(input.AssigneeIdentityID)
	if input.Scope == "" {
		input.Scope = domain.InboxScopeAll
	}
	if input.Scope == domain.InboxScopeCustomer && input.CustomerView == "" {
		input.CustomerView = domain.CustomerInboxViewQueue
	}
	if (input.Scope != domain.InboxScopeAll && input.Scope != domain.InboxScopeCustomer && input.Scope != domain.InboxScopeInternal) ||
		(input.Scope == domain.InboxScopeCustomer && input.CustomerView != domain.CustomerInboxViewQueue && input.CustomerView != domain.CustomerInboxViewMine && input.CustomerView != domain.CustomerInboxViewCoworkers && input.CustomerView != domain.CustomerInboxViewClosed) ||
		(input.AssigneeIdentityID != "" && (input.Scope != domain.InboxScopeCustomer || input.CustomerView != domain.CustomerInboxViewCoworkers || !common.ValidUUID(input.AssigneeIdentityID))) {
		return input, ErrQueryInvalid
	}

	if input.Scope != domain.InboxScopeCustomer {
		input.CustomerView = domain.CustomerInboxViewQueue
	}
	return input, nil
}
