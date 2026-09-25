//go:build server

package knowledgegap

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ListInput 定义待补知识清单的渠道、处理状态与分页，ChannelID 为空表示全部渠道。
type ListInput struct {
	ChannelID string
	Status    domain.KnowledgeGapStatus
	Page      int
	PageSize  int
}

// Summary 定义清单中的一条待补知识；Question 优先取 AI 起草的问题，其次为客户提问原文，HasDraft 表示 AI 已起草问答。
type Summary struct {
	ID                string    `bun:"id"`
	ConversationID    string    `bun:"conversation_id"`
	QuestionMessageID *string   `bun:"question_message_id"`
	Question          string    `bun:"question"`
	Source            string    `bun:"source"`
	Status            string    `bun:"status"`
	CategoryName      *string   `bun:"category_name"`
	HasDraft          bool      `bun:"has_draft"`
	OccurredAt        time.Time `bun:"occurred_at"`
}

// List 定义一页待补知识与总条数。
type List struct {
	Gaps     []Summary
	Page     int
	PageSize int
	Total    int
}

// ListQuery 读取待补知识清单。
type ListQuery struct{ db *bun.DB }

// NewListQuery 创建待补知识清单查询。
func NewListQuery(db *bun.DB) *ListQuery { return &ListQuery{db: db} }

// Execute 按来源发生时间倒序返回一页指定处理状态的待补知识。
func (q *ListQuery) Execute(ctx context.Context, identity *servermodels.Identity, input ListInput) (*List, error) {
	page, pageSize, valid := common.NormalizePagination(input.Page, input.PageSize)
	if !valid {
		return nil, ErrPageSizeInvalid
	}
	if !slices.Contains([]domain.KnowledgeGapStatus{domain.KnowledgeGapStatusPending, domain.KnowledgeGapStatusAccepted, domain.KnowledgeGapStatusDismissed}, input.Status) {
		return nil, ErrStatusInvalid
	}
	list := &List{Gaps: []Summary{}, Page: page, PageSize: pageSize}
	query := scoped(q.db.NewSelect(), identity.Organization.ID, input.ChannelID).
		Where("kg.status = ?", input.Status).
		Join("LEFT JOIN service_categories AS sc ON sc.id = ss.category_id AND sc.organization_id = ss.organization_id").
		Join("LEFT JOIN messages AS qm ON qm.id = kg.question_message_id AND qm.organization_id = kg.organization_id").
		ColumnExpr("kg.id, kg.conversation_id, kg.question_message_id, kg.source, kg.status, kg.occurred_at, sc.name AS category_name").
		ColumnExpr("coalesce(kg.draft_question, CASE WHEN qm.deleted_at IS NULL THEN qm.body END, '') AS question").
		ColumnExpr("kg.draft_status = ? AS has_draft", domain.KnowledgeGapDraftStatusReady).
		OrderExpr("kg.occurred_at DESC, kg.id DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize)
	total, err := query.ScanAndCount(ctx, &list.Gaps)
	if err != nil {
		return nil, fmt.Errorf("list knowledge gaps: %w", err)
	}
	list.Total = total
	return list, nil
}

// PendingCount 返回指定渠道下全部待处理的待补知识条数，channelID 为空表示全部渠道。
func PendingCount(ctx context.Context, db bun.IDB, organizationID, channelID string) (int, error) {
	count, err := scoped(db.NewSelect(), organizationID, channelID).
		Where("kg.status = ?", domain.KnowledgeGapStatusPending).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count pending knowledge gaps: %w", err)
	}
	return count, nil
}

// scoped 限定当前企业与渠道，并关联来源周期与渠道会话的渠道身份供调用方继续取列；非渠道来源的周期渠道身份为空。
func scoped(query *bun.SelectQuery, organizationID, channelID string) *bun.SelectQuery {
	query = query.TableExpr("knowledge_gaps AS kg").
		Join("JOIN service_sessions AS ss ON ss.id = kg.service_session_id AND ss.organization_id = kg.organization_id").
		Join("LEFT JOIN channel_conversations AS cc ON cc.conversation_id = ss.conversation_id AND cc.organization_id = ss.organization_id").
		Join("LEFT JOIN contact_channel_identities AS cci ON cci.id = cc.contact_channel_identity_id AND cci.organization_id = cc.organization_id").
		Where("kg.organization_id = ?", organizationID)
	if channelID != "" {
		query = query.Where("cci.channel_id = ?", channelID)
	}
	return query
}
