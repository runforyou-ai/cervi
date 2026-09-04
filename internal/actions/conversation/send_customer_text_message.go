//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

var memberMessageRetryableConstraintNames = map[string]struct{}{
	"chat_subjects_organization_kind_source_unique":             {},
	"conversation_participants_org_conversation_subject_unique": {},
	"messages_organization_idempotency_unique":                  {},
}

// SendCustomerTextMessageAction 持久化企业成员的客户会话文本回复。
type SendCustomerTextMessageAction struct {
	db *bun.DB
}

type memberMessageIDs struct {
	subject     string
	participant string
	message     string
}

type memberReplySessionPlan struct {
	assign bool
}

type idempotentMemberMessageRow struct {
	ID                     string     `bun:"id"`
	CreatedAt              time.Time  `bun:"created_at"`
	ConversationID         string     `bun:"conversation_id"`
	ServiceSessionID       *string    `bun:"service_session_id"`
	SenderParticipantID    *string    `bun:"sender_participant_id"`
	Type                   string     `bun:"type"`
	Body                   string     `bun:"body"`
	OriginatedAt           time.Time  `bun:"originated_at"`
	DeletedAt              *time.Time `bun:"deleted_at"`
	SenderSubjectID        *string    `bun:"sender_subject_id"`
	SenderSubjectKind      *string    `bun:"sender_subject_kind"`
	SenderSubjectSourceID  *string    `bun:"sender_subject_source_id"`
	JoinedServiceSessionID *string    `bun:"joined_service_session_id"`
}

// NewSendCustomerTextMessageAction 创建成员客户会话回复操作。
func NewSendCustomerTextMessageAction(db *bun.DB) *SendCustomerTextMessageAction {
	return &SendCustomerTextMessageAction{db: db}
}

// Execute 在一个可重试事务中写入成员客户会话回复。
func (a *SendCustomerTextMessageAction) Execute(ctx context.Context, identity *servermodels.Identity, input CustomerTextMessageInput) (ConversationMessage, error) {
	normalized, fields := normalizeCustomerTextMessageInput(input)
	if len(fields) > 0 {
		return ConversationMessage{}, &ValidationError{Fields: fields}
	}
	// 预生成一次事务重试期间稳定使用的 UUIDv7。
	values := make([]string, 3)
	for index := range values {
		values[index] = uuid.NewV7().String()
	}
	ids := memberMessageIDs{subject: values[0], participant: values[1], message: values[2]}
	var err error
	originatedAt := time.Now().UTC()
	idempotencyKey := "mmsg:" + identity.OrganizationIdentity.ID + ":" + normalized.ClientMessageID

	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		var result ConversationMessage
		err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			var executeErr error
			result, executeErr = a.executeTransaction(ctx, tx, identity, normalized, ids, originatedAt, idempotencyKey)
			return executeErr
		})
		if err == nil {
			return result, nil
		}
		constraint, retryable := retryableUniqueViolation(err, memberMessageRetryableConstraintNames)
		if !retryable {
			return ConversationMessage{}, err
		}
		if attempt < maxWriteAttempts-1 {
			slog.Info("成员客户消息写入重试", "conversation_id", normalized.ConversationID, "attempt", attempt+2, "constraint", constraint)
		}
	}
	slog.Warn("成员客户消息写入重试耗尽", "conversation_id", normalized.ConversationID, "error", err)
	return ConversationMessage{}, fmt.Errorf("send customer message retries exhausted: %w", err)
}

// executeTransaction 执行一次完整的成员客户会话回复事务。
func (a *SendCustomerTextMessageAction) executeTransaction(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, input CustomerTextMessageInput, ids memberMessageIDs, originatedAt time.Time, idempotencyKey string) (ConversationMessage, error) {
	if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
		return ConversationMessage{}, err
	}
	conversation, err := loadCustomerConversationForReply(ctx, tx, identity.Organization.ID, input.ConversationID)
	if err != nil {
		return ConversationMessage{}, err
	}
	if err := ensureCustomerConversationOutboundSupported(ctx, tx, identity.Organization.ID, conversation.ID); err != nil {
		return ConversationMessage{}, err
	}
	session, err := lockCurrentServiceSession(ctx, tx, identity.Organization.ID, conversation.ID)
	if err != nil {
		return ConversationMessage{}, err
	}
	if saved, found, err := loadIdempotentMemberMessage(ctx, tx, identity, input.ConversationID, input.Body, idempotencyKey, true); err != nil || found {
		return saved, err
	}

	// 计算成员回复对应的客服周期状态迁移。
	status := domain.ServiceSessionStatus(session.Status)
	if status == domain.ServiceSessionStatusClosed {
		return ConversationMessage{}, &ConflictError{Reason: ConflictReasonServiceSessionNotReplyable}
	}
	if status != domain.ServiceSessionStatusOpen {
		return ConversationMessage{}, ErrDataInvariant
	}
	if session.AssigneeIdentityID != nil && *session.AssigneeIdentityID != identity.OrganizationIdentity.ID {
		return ConversationMessage{}, &ConflictError{Reason: ConflictReasonServiceSessionOwned}
	}
	plan := memberReplySessionPlan{assign: session.AssigneeIdentityID == nil}
	if err := applyMemberReplySessionPlan(ctx, tx, session, identity.OrganizationIdentity.ID, originatedAt, plan); err != nil {
		return ConversationMessage{}, err
	}
	// 取得或创建当前企业成员的聊天主体。
	subject, err := ensureOrganizationIdentityChatSubject(ctx, tx, identity.Organization.ID, identity.OrganizationIdentity.ID, ids.subject)
	if err != nil {
		return ConversationMessage{}, err
	}
	participant, err := ensureMemberConversationParticipant(ctx, tx, identity.Organization.ID, conversation.ID, subject.ID, ids.participant)
	if err != nil {
		return ConversationMessage{}, err
	}

	message := &servermodels.Message{
		ID: ids.message, OrganizationID: identity.Organization.ID, ConversationID: conversation.ID,
		ServiceSessionID: &session.ID, SenderParticipantID: &participant.ID,
		Type: string(domain.MessageTypeText), Body: input.Body, IdempotencyKey: &idempotencyKey, OriginatedAt: originatedAt,
	}
	if _, err := tx.NewInsert().Model(message).
		Column("id", "organization_id", "conversation_id", "service_session_id", "sender_participant_id", "type", "body", "idempotency_key", "originated_at").
		Returning("*").
		Exec(ctx); err != nil {
		return ConversationMessage{}, fmt.Errorf("create member customer message: %w", err)
	}
	// 只记录客服处理周期的首次成员响应时间。
	if _, err := tx.NewUpdate().Model(session).
		Set("first_response_at = COALESCE(first_response_at, ?)", originatedAt).
		Set("updated_at = now()").
		WherePK().
		Where("organization_id = ?", session.OrganizationID).
		Exec(ctx); err != nil {
		return ConversationMessage{}, fmt.Errorf("record first member response: %w", err)
	}
	if err := updateSessionSummary(ctx, tx, session, message); err != nil {
		return ConversationMessage{}, err
	}
	if err := updateConversationSummary(ctx, tx, conversation, message); err != nil {
		return ConversationMessage{}, err
	}
	return memberConversationMessage(message, subject.ID, identity.OrganizationIdentity.ID, identity.OrganizationIdentity.DisplayName), nil
}

// ensureCustomerConversationOutboundSupported 校验客户会话来源渠道已实现外发。
func ensureCustomerConversationOutboundSupported(ctx context.Context, db bun.IDB, organizationID, conversationID string) error {
	var channelType string
	err := db.NewSelect().
		TableExpr("customer_conversations AS cc").
		ColumnExpr("ch.type").
		Join("JOIN contact_channel_identities AS cci ON cci.id = cc.contact_channel_identity_id AND cci.organization_id = cc.organization_id").
		Join("JOIN channels AS ch ON ch.id = cci.channel_id AND ch.organization_id = cci.organization_id").
		Where("cc.organization_id = ?", organizationID).
		Where("cc.conversation_id = ?", conversationID).
		Scan(ctx, &channelType)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDataInvariant
	}
	if err != nil {
		return fmt.Errorf("load customer conversation outbound channel: %w", err)
	}
	if domain.ChannelType(channelType) != domain.ChannelTypeWebsite {
		return &ConflictError{Reason: ConflictReasonChannelOutboundUnsupported}
	}
	return nil
}

// normalizeCustomerTextMessageInput 规范化并校验成员客户消息输入。
func normalizeCustomerTextMessageInput(input CustomerTextMessageInput) (CustomerTextMessageInput, map[string]ValidationCode) {
	fields := map[string]ValidationCode{}
	input.Body = strings.TrimSpace(input.Body)
	var valid bool
	input.ConversationID, valid = common.NormalizeUUID(input.ConversationID)
	if !valid {
		fields["conversationId"] = ValidationConversationIDInvalid
	}
	input.ClientMessageID, valid = common.NormalizeUUID(input.ClientMessageID)
	if !valid {
		fields["clientMessageId"] = ValidationClientMessageIDInvalid
	}
	if input.Body == "" {
		fields["body"] = ValidationBodyRequired
	} else if utf8.RuneCountInString(input.Body) > 4000 {
		fields["body"] = ValidationBodyTooLong
	}
	return input, fields
}

// loadCustomerConversationForReply 读取当前企业的客户会话。
func loadCustomerConversationForReply(ctx context.Context, db bun.IDB, organizationID, conversationID string) (*servermodels.Conversation, error) {
	conversation := &servermodels.Conversation{}
	err := db.NewSelect().Model(conversation).
		Join("JOIN customer_conversations AS cc ON cc.organization_id = cv.organization_id AND cc.conversation_id = cv.id").
		Where("cv.organization_id = ?", organizationID).
		Where("cv.id = ?", conversationID).
		Where("cv.type = ?", domain.ConversationTypeCustomer).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load customer conversation for reply: %w", err)
	}
	return conversation, nil
}

// lockCurrentServiceSession 锁定客户会话当前客服处理周期。
func lockCurrentServiceSession(ctx context.Context, db bun.IDB, organizationID, conversationID string) (*servermodels.ServiceSession, error) {
	customer := &servermodels.CustomerConversation{}
	err := db.NewSelect().Model(customer).
		Where("cc.organization_id = ?", organizationID).
		Where("cc.conversation_id = ?", conversationID).
		For("UPDATE").
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDataInvariant
	}
	if err != nil {
		return nil, fmt.Errorf("lock customer conversation: %w", err)
	}
	if customer.CurrentServiceSessionID == nil {
		return nil, ErrDataInvariant
	}
	session := &servermodels.ServiceSession{}
	err = db.NewSelect().Model(session).
		Where("ss.organization_id = ?", organizationID).
		Where("ss.conversation_id = ?", conversationID).
		Where("ss.id = ?", *customer.CurrentServiceSessionID).
		For("UPDATE").
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDataInvariant
	}
	if err != nil {
		return nil, fmt.Errorf("lock current service session: %w", err)
	}
	return session, nil
}

// loadIdempotentMemberMessage 校验并返回已经保存的成员消息。
func loadIdempotentMemberMessage(ctx context.Context, db bun.IDB, identity *servermodels.Identity, conversationID, body, idempotencyKey string, requireServiceSession bool) (ConversationMessage, bool, error) {
	row := idempotentMemberMessageRow{}
	err := db.NewSelect().
		TableExpr("messages AS msg").
		ColumnExpr("msg.id AS id").
		ColumnExpr("msg.created_at AS created_at").
		ColumnExpr("msg.conversation_id AS conversation_id").
		ColumnExpr("msg.service_session_id AS service_session_id").
		ColumnExpr("msg.sender_participant_id AS sender_participant_id").
		ColumnExpr("msg.type AS type").
		ColumnExpr("msg.body AS body").
		ColumnExpr("msg.originated_at AS originated_at").
		ColumnExpr("msg.deleted_at AS deleted_at").
		ColumnExpr("cs.id AS sender_subject_id").
		ColumnExpr("cs.kind AS sender_subject_kind").
		ColumnExpr("cs.source_id AS sender_subject_source_id").
		ColumnExpr("ss.id AS joined_service_session_id").
		Join("LEFT JOIN conversation_participants AS cp ON cp.id = msg.sender_participant_id AND cp.organization_id = msg.organization_id AND cp.conversation_id = msg.conversation_id").
		Join("LEFT JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id").
		Join("LEFT JOIN service_sessions AS ss ON ss.id = msg.service_session_id AND ss.organization_id = msg.organization_id AND ss.conversation_id = msg.conversation_id").
		Where("msg.organization_id = ?", identity.Organization.ID).
		Where("msg.idempotency_key = ?", idempotencyKey).
		Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return ConversationMessage{}, false, nil
	}
	if err != nil {
		return ConversationMessage{}, false, fmt.Errorf("load idempotent member message: %w", err)
	}
	// 校验幂等消息对应完整的成员发送意图。
	serviceSessionMatches := row.ServiceSessionID == nil && row.JoinedServiceSessionID == nil
	if requireServiceSession {
		serviceSessionMatches = row.ServiceSessionID != nil && row.JoinedServiceSessionID != nil && *row.ServiceSessionID == *row.JoinedServiceSessionID
	}
	messageMatches := row.ConversationID == conversationID && row.Body == body && row.Type == string(domain.MessageTypeText) && row.DeletedAt == nil &&
		serviceSessionMatches &&
		row.SenderParticipantID != nil && row.SenderSubjectID != nil && row.SenderSubjectKind != nil && row.SenderSubjectSourceID != nil &&
		*row.SenderSubjectKind == string(domain.ChatSubjectKindOrganizationIdentity) && *row.SenderSubjectSourceID == identity.OrganizationIdentity.ID
	if !messageMatches {
		return ConversationMessage{}, true, &ConflictError{Reason: ConflictReasonIdempotencyMismatch}
	}
	message := &servermodels.Message{
		ID: row.ID, CreatedAt: row.CreatedAt, ConversationID: row.ConversationID,
		ServiceSessionID: row.ServiceSessionID, SenderParticipantID: row.SenderParticipantID,
		Type: row.Type, Body: row.Body, OriginatedAt: row.OriginatedAt, DeletedAt: row.DeletedAt,
	}
	return memberConversationMessage(message, *row.SenderSubjectID, identity.OrganizationIdentity.ID, identity.OrganizationIdentity.DisplayName), true, nil
}

// applyMemberReplySessionPlan 应用成员回复对应的客服周期状态迁移。
func applyMemberReplySessionPlan(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession, identityID string, now time.Time, plan memberReplySessionPlan) error {
	if !plan.assign {
		return nil
	}
	query := db.NewUpdate().Model(session).
		Set("updated_at = now()").
		WherePK().
		Where("organization_id = ?", session.OrganizationID)
	if plan.assign {
		query = query.
			Set("assignee_identity_id = ?", identityID).
			Set("assigned_at = COALESCE(assigned_at, ?)", now)
	}
	if _, err := query.Exec(ctx); err != nil {
		return fmt.Errorf("apply member reply service session: %w", err)
	}
	if plan.assign {
		session.AssigneeIdentityID = &identityID
		if session.AssignedAt == nil {
			session.AssignedAt = &now
		}
	}
	return nil
}

// ensureMemberConversationParticipant 取得、创建或恢复当前成员的会话参与者。
func ensureMemberConversationParticipant(ctx context.Context, db bun.IDB, organizationID, conversationID, subjectID, participantID string) (*servermodels.ConversationParticipant, error) {
	participant := &servermodels.ConversationParticipant{}
	err := db.NewSelect().Model(participant).
		Where("cp.organization_id = ?", organizationID).
		Where("cp.conversation_id = ?", conversationID).
		Where("cp.subject_id = ?", subjectID).
		Scan(ctx)
	if err == nil {
		if participant.LeftAt != nil {
			if _, err := db.NewUpdate().Model(participant).
				Set("left_at = NULL").
				Set("role = ?", domain.ConversationParticipantRoleMember).
				Set("updated_at = now()").
				WherePK().
				Where("organization_id = ?", organizationID).
				Exec(ctx); err != nil {
				return nil, fmt.Errorf("restore member conversation participant: %w", err)
			}
			participant.LeftAt = nil
			participant.Role = string(domain.ConversationParticipantRoleMember)
		}
		return participant, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("find member conversation participant: %w", err)
	}
	participant = &servermodels.ConversationParticipant{
		ID: participantID, OrganizationID: organizationID, ConversationID: conversationID,
		SubjectID: subjectID, Role: string(domain.ConversationParticipantRoleMember),
	}
	if _, err := db.NewInsert().Model(participant).
		Column("id", "organization_id", "conversation_id", "subject_id", "role").
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("create member conversation participant: %w", err)
	}
	return participant, nil
}

// memberConversationMessage 构造成员消息时间线结果。
func memberConversationMessage(message *servermodels.Message, subjectID, sourceID, displayName string) ConversationMessage {
	name := displayName
	return ConversationMessage{
		ID: message.ID, Type: domain.MessageTypeText, Body: message.Body,
		OriginatedAt: message.OriginatedAt, SourceOrder: message.SourceOrder, CreatedAt: message.CreatedAt, MentionAll: message.MentionAll,
		Sender: &ConversationMessageSender{
			ChatSubjectID: subjectID, Kind: domain.ChatSubjectKindOrganizationIdentity,
			SourceID: sourceID, DisplayName: &name,
		},
	}
}
