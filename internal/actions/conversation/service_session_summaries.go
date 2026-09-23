//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/actions/servicecategory"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const (
	// serviceSummaryListLimit 是客户历史周期小结的读取条数上限。
	serviceSummaryListLimit = 50
	// serviceSummaryMaxRunes 是客服填写小结的最大字符数。
	serviceSummaryMaxRunes = 2000
)

const (
	ValidationServiceSessionIDInvalid ValidationCode = "service_session_id_invalid"
	ValidationSummaryTooLong          ValidationCode = "summary_too_long"
	ValidationCategoryIDInvalid       ValidationCode = "category_id_invalid"
)

// ConflictReasonServiceSessionNotClosed 表示客服处理周期尚未关闭，不能修改小结。
const ConflictReasonServiceSessionNotClosed = "service_session_not_closed"

// ErrServiceSessionNotFound 表示客服处理周期不存在或当前身份无权访问。
var ErrServiceSessionNotFound = errors.New("service session not found")

// ServiceSessionSummary 表示一个已关闭客服处理周期的结束方式与小结。
type ServiceSessionSummary struct {
	ServiceSessionID string                              `bun:"id"`
	ConversationID   string                              `bun:"conversation_id"`
	ChannelType      domain.ChannelType                  `bun:"channel_type"`
	ChannelName      string                              `bun:"channel_name"`
	ClosedAt         time.Time                           `bun:"closed_at"`
	CloseReason      domain.ServiceSessionCloseReason    `bun:"close_reason"`
	Status           *domain.ServiceSessionSummaryStatus `bun:"summary_status"`
	Summary          *string                             `bun:"summary"`
	Resolved         *bool                               `bun:"resolved"`
	CategoryID       *string                             `bun:"category_id"`
	CategoryName     *string                             `bun:"category_name"`
	EditedAt         *time.Time                          `bun:"summary_edited_at"`
	EditedByName     *string                             `bun:"edited_by_name"`
}

// ServiceSummaries 表示客户会话当前周期的交接摘要与同一客户已关闭周期的小结。
type ServiceSummaries struct {
	Handoff  *domain.HandoffSummary
	Sessions []ServiceSessionSummary
}

// serviceSessionSummaryQuery 构造已关闭周期小结的查询，含渠道、咨询分类与最后修改人。
func serviceSessionSummaryQuery(db bun.IDB, organizationID string) *bun.SelectQuery {
	return db.NewSelect().
		TableExpr("service_sessions AS ss").
		ColumnExpr("ss.id, ss.conversation_id, ch.type AS channel_type, ch.name AS channel_name, ss.closed_at, ss.close_reason").
		ColumnExpr("ss.summary_status, ss.summary, ss.resolved, ss.category_id, sc.name AS category_name, ss.summary_edited_at, editor.display_name AS edited_by_name").
		Join("JOIN contact_channel_identities AS cci ON cci.id = ss.contact_channel_identity_id AND cci.organization_id = ss.organization_id").
		Join("JOIN channels AS ch ON ch.id = cci.channel_id AND ch.organization_id = cci.organization_id").
		Join("LEFT JOIN service_categories AS sc ON sc.id = ss.category_id AND sc.organization_id = ss.organization_id").
		Join("LEFT JOIN organization_identities AS editor ON editor.id = ss.summary_edited_by_identity_id AND editor.organization_id = ss.organization_id").
		Where("ss.organization_id = ? AND ss.status = ?", organizationID, domain.ServiceSessionStatusClosed)
}

// ListServiceSummariesQuery 读取客户会话的交接摘要与客户历史周期小结。
type ListServiceSummariesQuery struct{ db *bun.DB }

// NewListServiceSummariesQuery 创建客户周期小结查询。
func NewListServiceSummariesQuery(db *bun.DB) *ListServiceSummariesQuery {
	return &ListServiceSummariesQuery{db: db}
}

// Execute 返回当前开放周期的交接摘要，以及同一客户在各渠道已关闭周期的小结，按关闭时间从新到旧排列。
func (q *ListServiceSummariesQuery) Execute(ctx context.Context, identity *servermodels.Identity, conversationID string) (ServiceSummaries, error) {
	if !common.ValidUUID(conversationID) {
		return ServiceSummaries{}, ErrConversationNotFound
	}
	result := ServiceSummaries{Sessions: make([]ServiceSessionSummary, 0)}
	err := q.db.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		if err := authorizeConversationHistory(ctx, tx, identity, conversationID); err != nil {
			return err
		}
		var current struct {
			ContactID      string                 `bun:"contact_id"`
			HandoffSummary *domain.HandoffSummary `bun:"handoff_summary,type:jsonb"`
		}
		err := tx.NewSelect().
			TableExpr("customer_conversations AS cc").
			ColumnExpr("cci.contact_id").
			ColumnExpr("CASE WHEN ss.status = ? THEN ss.handoff_summary END AS handoff_summary", domain.ServiceSessionStatusOpen).
			Join("JOIN contact_channel_identities AS cci ON cci.id = cc.contact_channel_identity_id AND cci.organization_id = cc.organization_id").
			Join("JOIN service_sessions AS ss ON ss.id = cc.current_service_session_id AND ss.organization_id = cc.organization_id").
			Where("cc.organization_id = ? AND cc.conversation_id = ?", identity.Organization.ID, conversationID).
			Scan(ctx, &current)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrConversationNotFound
		}
		if err != nil {
			return fmt.Errorf("load customer conversation for summaries: %w", err)
		}
		result.Handoff = current.HandoffSummary
		if err := serviceSessionSummaryQuery(tx, identity.Organization.ID).
			Where("cci.contact_id = ?", current.ContactID).
			OrderExpr("ss.closed_at DESC, ss.id DESC").
			Limit(serviceSummaryListLimit).
			Scan(ctx, &result.Sessions); err != nil {
			return fmt.Errorf("load customer service session summaries: %w", err)
		}
		return nil
	})
	if err != nil {
		return ServiceSummaries{}, err
	}
	return result, nil
}

// UpdateServiceSessionSummaryInput 定义客服填写的小结、是否解决与咨询分类；Resolved 与 CategoryID 为空表示不标注。
type UpdateServiceSessionSummaryInput struct {
	ServiceSessionID string
	Summary          string
	Resolved         *bool
	CategoryID       *string
}

// UpdateServiceSessionSummaryAction 保存客服修改的周期小结。
type UpdateServiceSessionSummaryAction struct{ db *bun.DB }

// NewUpdateServiceSessionSummaryAction 创建周期小结修改操作。
func NewUpdateServiceSessionSummaryAction(db *bun.DB) *UpdateServiceSessionSummaryAction {
	return &UpdateServiceSessionSummaryAction{db: db}
}

// Execute 校验并保存已关闭周期的小结、是否解决与咨询分类，记录修改人；保存后该周期的小结不再由 AI 覆盖。
func (a *UpdateServiceSessionSummaryAction) Execute(ctx context.Context, identity *servermodels.Identity, input UpdateServiceSessionSummaryInput) (ServiceSessionSummary, error) {
	fields := map[string]ValidationCode{}
	var valid bool
	input.ServiceSessionID, valid = common.NormalizeUUID(input.ServiceSessionID)
	if !valid {
		fields["serviceSessionId"] = ValidationServiceSessionIDInvalid
	}
	input.Summary = strings.TrimSpace(input.Summary)
	if utf8.RuneCountInString(input.Summary) > serviceSummaryMaxRunes {
		fields["summary"] = ValidationSummaryTooLong
	}
	if input.CategoryID != nil {
		categoryID, valid := common.NormalizeUUID(*input.CategoryID)
		if !valid {
			fields["categoryId"] = ValidationCategoryIDInvalid
		}
		input.CategoryID = &categoryID
	}
	if len(fields) > 0 {
		return ServiceSessionSummary{}, &ValidationError{Fields: fields}
	}
	var output ServiceSessionSummary
	err := realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		var conversationID string
		err := tx.NewSelect().Model((*servermodels.ServiceSession)(nil)).Column("conversation_id").
			Where("ss.organization_id = ? AND ss.id = ?", identity.Organization.ID, input.ServiceSessionID).
			Scan(ctx, &conversationID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrServiceSessionNotFound
		}
		if err != nil {
			return fmt.Errorf("load service session conversation: %w", err)
		}
		if err := authorizeConversationHistory(ctx, tx, identity, conversationID); err != nil {
			return err
		}
		conversation, err := chatstate.LockConversation(ctx, tx, identity.Organization.ID, conversationID)
		if err != nil {
			return err
		}
		session := &servermodels.ServiceSession{}
		if err := tx.NewSelect().Model(session).
			Where("ss.organization_id = ? AND ss.id = ?", identity.Organization.ID, input.ServiceSessionID).
			For("UPDATE").Scan(ctx); err != nil {
			return fmt.Errorf("lock service session: %w", err)
		}
		if domain.ServiceSessionStatus(session.Status) != domain.ServiceSessionStatusClosed {
			return &ConflictError{Reason: ConflictReasonServiceSessionNotClosed}
		}
		// 保留周期当前的咨询分类时不要求分类未归档。
		if input.CategoryID != nil && (session.CategoryID == nil || *session.CategoryID != *input.CategoryID) {
			category, err := servicecategory.FindActive(ctx, tx, identity.Organization.ID, *input.CategoryID)
			if err != nil {
				return err
			}
			if category == nil {
				return &ValidationError{Fields: map[string]ValidationCode{"categoryId": ValidationCategoryIDInvalid}}
			}
		}
		// 空白小结按未填写保存；无实质诉求的周期保存时未填写任何内容则保持该标记。
		var summary *string
		if input.Summary != "" {
			summary = &input.Summary
		}
		status := domain.ServiceSessionSummaryReady
		if summary == nil && input.Resolved == nil && input.CategoryID == nil && session.SummaryStatus != nil &&
			domain.ServiceSessionSummaryStatus(*session.SummaryStatus) == domain.ServiceSessionSummaryNoRequest {
			status = domain.ServiceSessionSummaryNoRequest
		}
		if _, err := tx.NewUpdate().Model(session).
			Set("summary_status = ?", status).
			Set("summary = ?", summary).
			Set("resolved = ?", input.Resolved).
			Set("category_id = ?", input.CategoryID).
			Set("summary_edited_by_identity_id = ?", identity.OrganizationIdentity.ID).
			Set("summary_edited_at = now()").
			WherePK().Where("organization_id = ?", identity.Organization.ID).
			Exec(ctx); err != nil {
			return fmt.Errorf("save service session summary: %w", err)
		}
		if err := chatstate.TouchConversation(ctx, tx, conversation); err != nil {
			return err
		}
		return serviceSessionSummaryQuery(tx, identity.Organization.ID).Where("ss.id = ?", session.ID).Scan(ctx, &output)
	})
	if err != nil {
		return ServiceSessionSummary{}, fmt.Errorf("update service session summary: %w", err)
	}
	return output, nil
}
