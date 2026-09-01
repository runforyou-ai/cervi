//go:build server

// Package inbox 实现统一收件箱领域的应用查询。
package inbox

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const inboxConversationTypeLimit = 50

// LoadInput 定义统一收件箱范围和客户队列筛选。
type LoadInput struct {
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
	Title                string
	ContactName          *string
	ContactAvatarFileID  *string
	ChannelType          domain.ChannelType
	ChannelName          string
	Preview              *string
	LastMessageAt        *time.Time
	ServiceSessionStatus domain.ServiceSessionStatus
	ServiceSessionID     string
	Assignee             *AssigneeSummary
}

// DirectConversationSummary 定义收件箱中的内部单聊详情。
type DirectConversationSummary struct {
	PeerIdentityID string
	PeerType       domain.OrganizationIdentityType
	PeerName       string
	Preview        *string
	LastMessageAt  *time.Time
	AgentRunStatus *domain.AgentRunStatus
}

// GroupConversationSummary 定义收件箱中的企业群聊详情。
type GroupConversationSummary struct {
	Title         string
	Preview       *string
	LastMessageAt *time.Time
	MemberCount   int
}

// ConversationSummary 定义统一收件箱会话信封。
type ConversationSummary struct {
	ID       string
	Type     domain.ConversationType
	Customer *CustomerConversationSummary
	Direct   *DirectConversationSummary
	Group    *GroupConversationSummary
	sortAt   *time.Time
}

// LoadInboxQuery 读取当前企业的统一收件箱。
type LoadInboxQuery struct {
	db *bun.DB
}

type customerConversationRow struct {
	ID                   string     `bun:"id"`
	Title                string     `bun:"title"`
	ContactName          *string    `bun:"contact_name"`
	ContactAvatarFileID  *string    `bun:"contact_avatar_file_id"`
	ChannelType          string     `bun:"channel_type"`
	ChannelName          string     `bun:"channel_name"`
	Preview              *string    `bun:"preview"`
	LastMessageAt        *time.Time `bun:"last_message_at"`
	ServiceSessionStatus string     `bun:"service_session_status"`
	ServiceSessionID     string     `bun:"service_session_id"`
	AssigneeIdentityID   *string    `bun:"assignee_identity_id"`
	AssigneeType         *string    `bun:"assignee_type"`
	AssigneeDisplayName  *string    `bun:"assignee_display_name"`
	AssigneeAvatarFileID *string    `bun:"assignee_avatar_file_id"`
	SortAt               *time.Time `bun:"sort_at"`
}

type directConversationRow struct {
	ID             string     `bun:"id"`
	PeerIdentityID string     `bun:"peer_identity_id"`
	PeerType       string     `bun:"peer_type"`
	PeerName       string     `bun:"peer_name"`
	Preview        *string    `bun:"preview"`
	LastMessageAt  *time.Time `bun:"last_message_at"`
	AgentRunStatus *string    `bun:"agent_run_status"`
	SortAt         *time.Time `bun:"sort_at"`
}

type groupConversationRow struct {
	ID            string     `bun:"id"`
	Title         string     `bun:"title"`
	Preview       *string    `bun:"preview"`
	LastMessageAt *time.Time `bun:"last_message_at"`
	MemberCount   int        `bun:"member_count"`
	SortAt        *time.Time `bun:"sort_at"`
}

// NewLoadInboxQuery 创建成员收件箱查询。
func NewLoadInboxQuery(db *bun.DB) *LoadInboxQuery {
	return &LoadInboxQuery{db: db}
}

// Execute 分别读取客户会话、内部单聊和群聊后合并排序。
func (q *LoadInboxQuery) Execute(ctx context.Context, identity *servermodels.Identity, input LoadInput) ([]ConversationSummary, error) {
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
		return nil, ErrQueryInvalid
	}

	customers := make([]customerConversationRow, 0)
	directs := make([]directConversationRow, 0)
	groups := make([]groupConversationRow, 0)
	var err error
	if input.Scope != domain.InboxScopeInternal {
		customers, err = q.loadCustomerConversations(ctx, identity.Organization.ID, identity.OrganizationIdentity.ID, input)
		if err != nil {
			return nil, err
		}
	}
	if input.Scope != domain.InboxScopeCustomer {
		directs, err = q.loadDirectConversations(ctx, identity.Organization.ID, identity.OrganizationIdentity.ID)
		if err != nil {
			return nil, err
		}
		groups, err = q.loadGroupConversations(ctx, identity.Organization.ID, identity.OrganizationIdentity.ID)
		if err != nil {
			return nil, err
		}
	}

	result := make([]ConversationSummary, 0, len(customers)+len(directs)+len(groups))
	for _, row := range customers {
		var assignee *AssigneeSummary
		if row.AssigneeIdentityID != nil && row.AssigneeType != nil && row.AssigneeDisplayName != nil {
			assignee = &AssigneeSummary{IdentityID: *row.AssigneeIdentityID, Type: domain.OrganizationIdentityType(*row.AssigneeType), DisplayName: *row.AssigneeDisplayName, AvatarFileID: row.AssigneeAvatarFileID}
		}
		result = append(result, ConversationSummary{
			ID: row.ID, Type: domain.ConversationTypeCustomer, sortAt: row.SortAt,
			Customer: &CustomerConversationSummary{
				Title: row.Title, ContactName: row.ContactName, ContactAvatarFileID: row.ContactAvatarFileID,
				ChannelType: domain.ChannelType(row.ChannelType), ChannelName: row.ChannelName,
				Preview: row.Preview, LastMessageAt: row.LastMessageAt,
				ServiceSessionID: row.ServiceSessionID, ServiceSessionStatus: domain.ServiceSessionStatus(row.ServiceSessionStatus), Assignee: assignee,
			},
		})
	}
	for _, row := range directs {
		var agentRunStatus *domain.AgentRunStatus
		if row.AgentRunStatus != nil {
			status := domain.AgentRunStatus(*row.AgentRunStatus)
			agentRunStatus = &status
		}
		result = append(result, ConversationSummary{
			ID: row.ID, Type: domain.ConversationTypeDirect, sortAt: row.SortAt,
			Direct: &DirectConversationSummary{
				PeerIdentityID: row.PeerIdentityID, PeerType: domain.OrganizationIdentityType(row.PeerType), PeerName: row.PeerName,
				Preview: row.Preview, LastMessageAt: row.LastMessageAt, AgentRunStatus: agentRunStatus,
			},
		})
	}
	for _, row := range groups {
		result = append(result, ConversationSummary{
			ID: row.ID, Type: domain.ConversationTypeGroup, sortAt: row.SortAt,
			Group: &GroupConversationSummary{
				Title: row.Title, Preview: row.Preview,
				LastMessageAt: row.LastMessageAt, MemberCount: row.MemberCount,
			},
		})
	}
	sort.Slice(result, func(first, second int) bool {
		firstTime := result[first].sortAt
		secondTime := result[second].sortAt
		if firstTime == nil || secondTime == nil {
			if firstTime == nil && secondTime == nil {
				return result[first].ID > result[second].ID
			}
			return firstTime != nil
		}
		if firstTime.Equal(*secondTime) {
			return result[first].ID > result[second].ID
		}
		return firstTime.After(*secondTime)
	})
	return result, nil
}

// loadCustomerConversations 按客户视图读取最新处理周期对应的会话。
func (q *LoadInboxQuery) loadCustomerConversations(ctx context.Context, organizationID, currentIdentityID string, input LoadInput) ([]customerConversationRow, error) {
	var rows []customerConversationRow
	query := q.db.NewSelect().
		TableExpr("customer_conversations AS cc").
		ColumnExpr("cv.id AS id").
		ColumnExpr("cv.title AS title").
		ColumnExpr("COALESCE(cci.display_name, c.display_name) AS contact_name").
		ColumnExpr("cci.avatar_file_id AS contact_avatar_file_id").
		ColumnExpr("ch.type AS channel_type").
		ColumnExpr("ch.name AS channel_name").
		ColumnExpr("msg.body AS preview").
		ColumnExpr("cv.last_message_at AS last_message_at").
		ColumnExpr("latest.status AS service_session_status").
		ColumnExpr("latest.id::text AS service_session_id").
		ColumnExpr("latest.assignee_identity_id::text AS assignee_identity_id").
		ColumnExpr("assignee.type AS assignee_type").
		ColumnExpr("assignee.display_name AS assignee_display_name").
		ColumnExpr("assignee.avatar_file_id::text AS assignee_avatar_file_id").
		ColumnExpr("cv.last_message_at AS sort_at").
		Join("JOIN conversations AS cv ON cv.id = cc.conversation_id AND cv.organization_id = cc.organization_id").
		Join("JOIN contact_channel_identities AS cci ON cci.id = cc.contact_channel_identity_id AND cci.organization_id = cc.organization_id").
		Join("JOIN contacts AS c ON c.id = cci.contact_id AND c.organization_id = cc.organization_id").
		Join("JOIN channels AS ch ON ch.id = cci.channel_id AND ch.organization_id = cc.organization_id").
		Join("JOIN messages AS msg ON msg.id = cv.last_message_id AND msg.organization_id = cv.organization_id AND msg.conversation_id = cv.id AND msg.deleted_at IS NULL").
		Join("JOIN LATERAL (SELECT ss.id, ss.status, ss.assignee_identity_id FROM service_sessions AS ss WHERE ss.organization_id = cv.organization_id AND ss.conversation_id = cv.id ORDER BY ss.sequence DESC LIMIT 1) AS latest ON TRUE").
		Join("LEFT JOIN organization_identities AS assignee ON assignee.organization_id = cv.organization_id AND assignee.id = latest.assignee_identity_id").
		Where("cc.organization_id = ?", organizationID).
		Where("cv.type = ?", domain.ConversationTypeCustomer)
	if input.Scope == domain.InboxScopeAll {
		query = query.
			Where("latest.status = ?", domain.ServiceSessionStatusOpen).
			Where(`(
				latest.assignee_identity_id = ?
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
			query = query.Where("latest.status = ?", domain.ServiceSessionStatusOpen).Where("latest.assignee_identity_id IS NULL")
		case domain.CustomerInboxViewMine:
			query = query.Where("latest.status = ?", domain.ServiceSessionStatusOpen).Where("latest.assignee_identity_id = ?", currentIdentityID)
		case domain.CustomerInboxViewCoworkers:
			query = query.Where("latest.status = ?", domain.ServiceSessionStatusOpen).
				Where("latest.assignee_identity_id IS NOT NULL").
				Where("latest.assignee_identity_id <> ?", currentIdentityID)
			if input.AssigneeIdentityID != "" {
				query = query.Where("latest.assignee_identity_id = ?", input.AssigneeIdentityID)
			}
		case domain.CustomerInboxViewClosed:
			query = query.Where("latest.status = ?", domain.ServiceSessionStatusClosed)
		}
	}
	err := query.OrderExpr("cv.last_message_at DESC NULLS LAST, cv.id DESC").
		Limit(inboxConversationTypeLimit).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("list customer inbox conversations: %w", err)
	}
	return rows, nil
}

// loadDirectConversations 读取当前成员参与的内部单聊，包括尚无消息的新会话。
func (q *LoadInboxQuery) loadDirectConversations(ctx context.Context, organizationID, identityID string) ([]directConversationRow, error) {
	var rows []directConversationRow
	err := q.db.NewSelect().
		TableExpr("conversations AS cv").
		ColumnExpr("cv.id AS id").
		ColumnExpr("peer_cs.source_id AS peer_identity_id").
		ColumnExpr("peer_oi.type AS peer_type").
		ColumnExpr("peer_oi.display_name AS peer_name").
		ColumnExpr("msg.body AS preview").
		ColumnExpr("cv.last_message_at AS last_message_at").
		ColumnExpr("latest_agent_run.status AS agent_run_status").
		ColumnExpr("cv.last_message_at AS sort_at").
		Join("JOIN conversation_participants AS mine ON mine.organization_id = cv.organization_id AND mine.conversation_id = cv.id AND mine.left_at IS NULL").
		Join("JOIN chat_subjects AS mine_cs ON mine_cs.organization_id = mine.organization_id AND mine_cs.id = mine.subject_id AND mine_cs.kind = ? AND mine_cs.source_id = ?", domain.ChatSubjectKindOrganizationIdentity, identityID).
		Join("JOIN conversation_participants AS peer ON peer.organization_id = cv.organization_id AND peer.conversation_id = cv.id AND peer.subject_id <> mine.subject_id AND peer.left_at IS NULL").
		Join("JOIN chat_subjects AS peer_cs ON peer_cs.organization_id = peer.organization_id AND peer_cs.id = peer.subject_id AND peer_cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("JOIN organization_identities AS peer_oi ON peer_oi.organization_id = peer_cs.organization_id AND peer_oi.id = peer_cs.source_id").
		Join("LEFT JOIN users AS peer_u ON peer_u.organization_id = peer_oi.organization_id AND peer_u.identity_id = peer_oi.id").
		Join("LEFT JOIN agents AS peer_a ON peer_a.organization_id = peer_oi.organization_id AND peer_a.identity_id = peer_oi.id").
		Join("LEFT JOIN messages AS msg ON msg.organization_id = cv.organization_id AND msg.conversation_id = cv.id AND msg.id = cv.last_message_id AND msg.deleted_at IS NULL").
		Join("LEFT JOIN LATERAL (SELECT agr.status FROM agent_runs AS agr WHERE agr.organization_id = cv.organization_id AND agr.conversation_id = cv.id AND agr.agent_identity_id = peer_oi.id ORDER BY agr.created_at DESC, agr.id DESC LIMIT 1) AS latest_agent_run ON peer_oi.type = ?", domain.OrganizationIdentityTypeAgent).
		Where("cv.organization_id = ?", organizationID).
		Where("cv.type = ?", domain.ConversationTypeDirect).
		Where("cv.status = ?", domain.ConversationStatusActive).
		Where("((peer_oi.type = ? AND peer_u.status = ?) OR (peer_oi.type = ? AND peer_a.status = ?))", domain.OrganizationIdentityTypeUser, domain.UserStatusActive, domain.OrganizationIdentityTypeAgent, domain.UserStatusActive).
		Where("(SELECT count(*) FROM conversation_participants AS all_cp WHERE all_cp.organization_id = cv.organization_id AND all_cp.conversation_id = cv.id) = 2").
		OrderExpr("cv.last_message_at DESC NULLS LAST, cv.id DESC").
		Limit(inboxConversationTypeLimit).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("list direct inbox conversations: %w", err)
	}
	return rows, nil
}

// loadGroupConversations 读取当前成员参与的企业群聊，包括尚无消息的新群聊。
func (q *LoadInboxQuery) loadGroupConversations(ctx context.Context, organizationID, identityID string) ([]groupConversationRow, error) {
	var rows []groupConversationRow
	err := q.db.NewSelect().
		TableExpr("conversations AS cv").
		ColumnExpr("cv.id AS id").
		ColumnExpr("cv.title AS title").
		ColumnExpr("msg.body AS preview").
		ColumnExpr("cv.last_message_at AS last_message_at").
		ColumnExpr("members.member_count AS member_count").
		ColumnExpr("cv.last_message_at AS sort_at").
		Join("JOIN conversation_participants AS mine ON mine.organization_id = cv.organization_id AND mine.conversation_id = cv.id AND mine.left_at IS NULL").
		Join("JOIN chat_subjects AS mine_cs ON mine_cs.organization_id = mine.organization_id AND mine_cs.id = mine.subject_id AND mine_cs.kind = ? AND mine_cs.source_id = ?", domain.ChatSubjectKindOrganizationIdentity, identityID).
		Join("JOIN LATERAL (SELECT count(*) AS member_count FROM conversation_participants AS member_cp WHERE member_cp.organization_id = cv.organization_id AND member_cp.conversation_id = cv.id AND member_cp.left_at IS NULL) AS members ON TRUE").
		Join("LEFT JOIN messages AS msg ON msg.organization_id = cv.organization_id AND msg.conversation_id = cv.id AND msg.id = cv.last_message_id AND msg.deleted_at IS NULL").
		Where("cv.organization_id = ?", organizationID).
		Where("cv.type = ?", domain.ConversationTypeGroup).
		Where("cv.status = ?", domain.ConversationStatusActive).
		OrderExpr("cv.last_message_at DESC NULLS LAST, cv.id DESC").
		Limit(inboxConversationTypeLimit).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("list group inbox conversations: %w", err)
	}
	return rows, nil
}
