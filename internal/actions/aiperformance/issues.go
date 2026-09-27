//go:build server

package aiperformance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/runforyou-ai/cervi/internal/actions/knowledgegap"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/storage/server/messagequery"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// issueConditions 是各问题类型在公共集合 closed 上的筛选条件，满意度取值由调用方按顺序传入。
var issueConditions = map[domain.AIPerformanceIssueType]string{
	domain.AIPerformanceIssueTypeAll:             "(closed.satisfaction = ? OR closed.ai_incorrect OR closed.ai_missed_handoff OR closed.ai_poor_attitude)",
	domain.AIPerformanceIssueTypeDissatisfied:    "closed.satisfaction = ?",
	domain.AIPerformanceIssueTypeAIIncorrect:     "closed.ai_incorrect",
	domain.AIPerformanceIssueTypeAIMissedHandoff: "closed.ai_missed_handoff",
	domain.AIPerformanceIssueTypeAIPoorAttitude:  "closed.ai_poor_attitude",
}

// IssueListInput 定义问题会话的统计范围、问题类型与分页。
type IssueListInput struct {
	Input
	Issue    domain.AIPerformanceIssueType
	Page     int
	PageSize int
}

// Issue 定义一个问题会话：满意度为不满意或任一质检标记成立的已关闭周期；Summary 只在小结已生成时有值，Preview 为周期首条消息摘要。
type Issue struct {
	ServiceSessionID      string    `bun:"id"`
	ConversationID        string    `bun:"conversation_id"`
	ChannelType           *string   `bun:"channel_type"`
	ChannelName           *string   `bun:"channel_name"`
	RequesterName         *string   `bun:"requester_name"`
	RequesterAvatarFileID *string   `bun:"requester_avatar_file_id"`
	ClosedAt              time.Time `bun:"closed_at"`
	Summary               *string   `bun:"summary"`
	Preview               string    `bun:"preview"`
	Satisfaction          *string   `bun:"satisfaction"`
	AIIncorrect           *bool     `bun:"ai_incorrect"`
	AIMissedHandoff       *bool     `bun:"ai_missed_handoff"`
	AIPoorAttitude        *bool     `bun:"ai_poor_attitude"`
}

// IssueList 定义一页问题会话与总条数。
type IssueList struct {
	Issues   []Issue
	Page     int
	PageSize int
	Total    int
}

// IssueDetail 定义问题会话详情：周期信息、质检结论与周期内的对客沟通。
type IssueDetail struct {
	Issue
	Messages []knowledgegap.Message
}

// issueSelectSQL 读取问题会话的展示字段，第一个 %s 拼入不含参数的来源集合 src，第二个拼入筛选与排序。
const issueSelectSQL = `
SELECT ss.id, ss.conversation_id, ch.type AS channel_type, ch.name AS channel_name,
	coalesce(cci.display_name, c.display_name, requester_oi.display_name) AS requester_name,
	coalesce(cci.avatar_file_id, requester_oi.avatar_file_id)::text AS requester_avatar_file_id,
	ss.closed_at, CASE WHEN ss.summary_status = ? THEN ss.summary END AS summary,
	coalesce(?, '') AS preview,
	ssr.satisfaction, ssr.ai_incorrect, ssr.ai_missed_handoff, ssr.ai_poor_attitude
FROM %s
JOIN service_sessions ss ON ss.id = src.id AND ss.organization_id = src.organization_id
JOIN service_session_reviews ssr ON ssr.organization_id = ss.organization_id AND ssr.service_session_id = ss.id
JOIN service_conversations svc ON svc.id = ss.service_conversation_id AND svc.organization_id = ss.organization_id
JOIN chat_subjects requester_cs ON requester_cs.id = svc.requester_subject_id AND requester_cs.organization_id = svc.organization_id
LEFT JOIN contacts c ON c.id = requester_cs.source_id AND c.organization_id = requester_cs.organization_id AND requester_cs.kind = ?
LEFT JOIN organization_identities requester_oi ON requester_oi.id = requester_cs.source_id AND requester_oi.organization_id = requester_cs.organization_id AND requester_cs.kind = ?
LEFT JOIN channel_conversations cc ON cc.conversation_id = svc.conversation_id AND cc.organization_id = svc.organization_id
LEFT JOIN contact_channel_identities cci ON cci.id = cc.contact_channel_identity_id AND cci.organization_id = cc.organization_id
LEFT JOIN channels ch ON ch.id = cci.channel_id AND ch.organization_id = cci.organization_id
LEFT JOIN messages opening ON opening.id = ss.opening_message_id AND opening.organization_id = ss.organization_id AND opening.deleted_at IS NULL
%s`

// issueSelectArgs 返回 issueSelectSQL 中展示字段与关联的参数。
func issueSelectArgs() []any {
	return []any{domain.ServiceSessionSummaryReady, messagequery.Summary("opening"), domain.ChatSubjectKindContact, domain.ChatSubjectKindOrganizationIdentity}
}

// IssueListQuery 读取问题会话。
type IssueListQuery struct{ db *bun.DB }

// NewIssueListQuery 创建问题会话查询。
func NewIssueListQuery(db *bun.DB) *IssueListQuery { return &IssueListQuery{db: db} }

// Execute 按关闭时间倒序返回统计范围内一页指定类型的问题会话。
func (q *IssueListQuery) Execute(ctx context.Context, identity *servermodels.Identity, input IssueListInput) (*IssueList, error) {
	page, pageSize, valid := common.NormalizePagination(input.Page, input.PageSize)
	if !valid {
		return nil, ErrPageSizeInvalid
	}
	condition, ok := issueConditions[input.Issue]
	if !ok {
		return nil, ErrIssueInvalid
	}
	// 包含满意度条件的类型需要传入不满意取值。
	var conditionArgs []any
	if input.Issue == domain.AIPerformanceIssueTypeAll || input.Issue == domain.AIPerformanceIssueTypeDissatisfied {
		conditionArgs = []any{domain.ServiceSessionSatisfactionDissatisfied}
	}
	scope, args := reportScope(identity, input.Input)
	scope += `, issues AS (SELECT closed.id, closed.organization_id, closed.closed_at FROM closed WHERE ` + condition + `)`
	scopeArgs := slices.Concat(args, conditionArgs)
	list := &IssueList{Issues: []Issue{}, Page: page, PageSize: pageSize}
	if err := q.db.NewRaw(scope+` SELECT count(*) FROM issues`, scopeArgs...).Scan(ctx, &list.Total); err != nil {
		return nil, fmt.Errorf("count ai performance issues: %w", err)
	}
	if err := q.db.NewRaw(scope+fmt.Sprintf(issueSelectSQL, "issues src", "ORDER BY src.closed_at DESC, src.id DESC LIMIT ? OFFSET ?"),
		slices.Concat(scopeArgs, issueSelectArgs(), []any{pageSize, (page - 1) * pageSize})...).
		Scan(ctx, &list.Issues); err != nil {
		return nil, fmt.Errorf("list ai performance issues: %w", err)
	}
	return list, nil
}

// IssueQuery 读取问题会话详情。
type IssueQuery struct{ db *bun.DB }

// NewIssueQuery 创建问题会话详情查询。
func NewIssueQuery(db *bun.DB) *IssueQuery { return &IssueQuery{db: db} }

// Execute 返回当前企业中已质检的已关闭周期的质检结论与对客沟通。
func (q *IssueQuery) Execute(ctx context.Context, identity *servermodels.Identity, serviceSessionID string) (*IssueDetail, error) {
	if !common.ValidUUID(serviceSessionID) {
		return nil, ErrIssueNotFound
	}
	organizationID := identity.Organization.ID
	detail := &IssueDetail{}
	err := q.db.NewRaw(fmt.Sprintf(issueSelectSQL, "service_sessions src", "WHERE src.organization_id = ? AND src.id = ? AND src.status = ?"),
		slices.Concat(issueSelectArgs(), []any{organizationID, serviceSessionID, domain.ServiceSessionStatusClosed})...).
		Scan(ctx, &detail.Issue)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrIssueNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load ai performance issue: %w", err)
	}
	if detail.Messages, err = knowledgegap.Transcript(ctx, q.db, organizationID, serviceSessionID); err != nil {
		return nil, err
	}
	return detail, nil
}
