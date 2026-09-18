//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	"github.com/runforyou-ai/cervi/internal/storage/server/messagequery"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/storage/server/pgerr"
	"github.com/uptrace/bun"
)

const websiteExternalIDPrefix = "web-session:"

// maxWriteAttempts 是并发唯一约束冲突时的最大写入尝试次数。
const maxWriteAttempts = 3

var websiteMessageRetryableConstraintNames = map[string]struct{}{
	"contact_channel_identities_channel_external_unique":         {},
	"chat_subjects_organization_kind_source_unique":              {},
	"conversation_participants_org_conversation_subject_unique":  {},
	"service_sessions_organization_conversation_open_unique":     {},
	"service_sessions_organization_conversation_sequence_unique": {},
	"messages_organization_idempotency_unique":                   {},
}

// ReceiveWebsiteCustomerMessageAction 持久化网站访客文本与附件消息。
type ReceiveWebsiteCustomerMessageAction struct {
	db             *bun.DB
	agentScheduler CustomerAgentMessageScheduler
}

// CustomerAgentMessageScheduler 把渠道客户消息加入当前 AI 客服的持久输入流。
type CustomerAgentMessageScheduler interface {
	ScheduleCustomerAuto(context.Context, bun.IDB, string, string, string, string) (bool, error)
}

// AgentMessageScheduler 同时调度内部单聊和渠道客户 Agent 输入。
type AgentMessageScheduler interface {
	AgentChatMessageScheduler
	CustomerAgentMessageScheduler
	GroupAgentMessageScheduler
}

type generatedIDs struct {
	contact         string
	channelIdentity string
	subject         string
	conversation    string
	participant     string
	serviceSession  string
	message         string
}

// NewReceiveWebsiteCustomerMessageAction 创建网站访客消息操作。
func NewReceiveWebsiteCustomerMessageAction(db *bun.DB, agentScheduler CustomerAgentMessageScheduler) *ReceiveWebsiteCustomerMessageAction {
	return &ReceiveWebsiteCustomerMessageAction{db: db, agentScheduler: agentScheduler}
}

// Execute 在一个可重试事务中写入网站访客文本消息。
func (a *ReceiveWebsiteCustomerMessageAction) Execute(ctx context.Context, input WebsiteCustomerTextMessageInput) (ReceiveWebsiteCustomerMessageResult, error) {
	normalized, fields := normalizeWebsiteMessageInput(input)
	if len(fields) > 0 {
		return ReceiveWebsiteCustomerMessageResult{}, &ValidationError{Fields: fields}
	}
	return a.receive(ctx, normalized.ChannelID, InboundCustomerMessageInput{
		ExternalID: normalized.ExternalID, RequestedConversationID: normalized.ConversationID,
		Body: normalized.Body, ClientMessageID: &normalized.ClientMessageID, ReplyToMessageID: normalized.ReplyToMessageID,
	})
}

// ExecuteAttachment 在一个可重试事务中写入网站访客附件消息并激活上传文件。
func (a *ReceiveWebsiteCustomerMessageAction) ExecuteAttachment(ctx context.Context, input WebsiteCustomerAttachmentMessageInput) (ReceiveWebsiteCustomerMessageResult, error) {
	normalized, fields := normalizeWebsiteAttachmentMessageInput(input)
	if len(fields) > 0 {
		return ReceiveWebsiteCustomerMessageResult{}, &ValidationError{Fields: fields}
	}
	return a.receive(ctx, normalized.ChannelID, InboundCustomerMessageInput{
		ExternalID: normalized.ExternalID, RequestedConversationID: normalized.ConversationID,
		Body: normalized.Body, ClientMessageID: &normalized.ClientMessageID, ReplyToMessageID: normalized.ReplyToMessageID,
		Attachment: &InboundCustomerAttachment{FileID: normalized.FileID, ImageWidth: normalized.ImageWidth, ImageHeight: normalized.ImageHeight},
	})
}

// receive 在可重试事务中写入访客入站消息。
func (a *ReceiveWebsiteCustomerMessageAction) receive(ctx context.Context, channelID string, input InboundCustomerMessageInput) (ReceiveWebsiteCustomerMessageResult, error) {
	var err error
	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		var result ReceiveWebsiteCustomerMessageResult
		err = realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
			var executeErr error
			result, executeErr = a.executeTransaction(ctx, tx, channelID, input)
			return executeErr
		})
		if err == nil {
			return result, nil
		}
		constraint, retryable := retryableUniqueViolation(err, websiteMessageRetryableConstraintNames)
		if !retryable {
			return ReceiveWebsiteCustomerMessageResult{}, err
		}
		if attempt < maxWriteAttempts-1 {
			slog.Info("网站访客消息写入重试", "channel_id", channelID, "attempt", attempt+2, "constraint", constraint)
		}
	}
	slog.Warn("网站访客消息写入重试耗尽", "channel_id", channelID, "error", err)
	return ReceiveWebsiteCustomerMessageResult{}, fmt.Errorf("receive website message retries exhausted: %w", err)
}

// executeTransaction 执行一次完整的网站访客消息事务。
func (a *ReceiveWebsiteCustomerMessageAction) executeTransaction(ctx context.Context, tx bun.Tx, channelID string, input InboundCustomerMessageInput) (ReceiveWebsiteCustomerMessageResult, error) {
	channel, err := loadWebsiteChannel(ctx, tx, channelID)
	if err != nil {
		return ReceiveWebsiteCustomerMessageResult{}, err
	}
	received, err := ReceiveInboundCustomerMessage(ctx, tx, channel, input)
	if err != nil {
		return ReceiveWebsiteCustomerMessageResult{}, err
	}
	if !received.Inserted {
		slog.Debug("网站访客消息幂等命中",
			"channel_id", channel.ID,
			"conversation_id", received.Message.ConversationID,
			"message_id", received.Message.ID,
		)
		return receiveWebsiteCustomerMessageResult(channel.OrganizationID, received), nil
	}
	if a.agentScheduler == nil {
		return ReceiveWebsiteCustomerMessageResult{}, errors.New("customer agent scheduler is unavailable")
	}
	if _, err := a.agentScheduler.ScheduleCustomerAuto(
		ctx, tx, channel.OrganizationID, received.Message.ConversationID, received.Session.ID, received.Message.ID,
	); err != nil {
		return ReceiveWebsiteCustomerMessageResult{}, fmt.Errorf("schedule website customer agent: %w", err)
	}
	return receiveWebsiteCustomerMessageResult(channel.OrganizationID, received), nil
}

// normalizeWebsiteMessageInput 规范化并校验网站消息输入。
func normalizeWebsiteMessageInput(input WebsiteCustomerTextMessageInput) (WebsiteCustomerTextMessageInput, map[string]ValidationCode) {
	fields := map[string]ValidationCode{}
	input.Body = strings.TrimSpace(input.Body)
	if !common.ValidUUID(input.ChannelID) {
		fields["channelId"] = ValidationChannelIDInvalid
	}
	if !validWebsiteExternalID(input.ExternalID) {
		fields["visitorToken"] = ValidationExternalIDInvalid
	}
	if input.ConversationID != nil && !common.ValidUUID(*input.ConversationID) {
		fields["conversationId"] = ValidationConversationIDInvalid
	}
	clientMessageID, valid := common.NormalizeUUID(input.ClientMessageID)
	input.ClientMessageID = clientMessageID
	if !valid {
		fields["clientMessageId"] = ValidationClientMessageIDInvalid
	}
	if input.ReplyToMessageID != "" {
		var valid bool
		input.ReplyToMessageID, valid = common.NormalizeUUID(input.ReplyToMessageID)
		if !valid || input.ConversationID == nil {
			fields["replyToMessageId"] = ValidationReplyToMessageIDInvalid
		}
	}
	if input.Body == "" {
		fields["body"] = ValidationBodyRequired
	} else if utf8.RuneCountInString(input.Body) > 4000 {
		fields["body"] = ValidationBodyTooLong
	}
	return input, fields
}

// normalizeWebsiteAttachmentMessageInput 规范化并校验网站访客附件消息输入。
func normalizeWebsiteAttachmentMessageInput(input WebsiteCustomerAttachmentMessageInput) (WebsiteCustomerAttachmentMessageInput, map[string]ValidationCode) {
	text, fields := normalizeWebsiteMessageInput(WebsiteCustomerTextMessageInput{
		ChannelID: input.ChannelID, ExternalID: input.ExternalID, ConversationID: input.ConversationID,
		ClientMessageID: input.ClientMessageID, Body: input.Body, ReplyToMessageID: input.ReplyToMessageID,
	})
	input.ChannelID, input.ExternalID, input.ConversationID = text.ChannelID, text.ExternalID, text.ConversationID
	input.ClientMessageID, input.Body, input.ReplyToMessageID = text.ClientMessageID, text.Body, text.ReplyToMessageID
	// 附件消息的说明可以为空，长度仍按渠道说明上限校验。
	if fields["body"] == ValidationBodyRequired {
		delete(fields, "body")
	}
	if utf8.RuneCountInString(input.Body) > domain.ChannelCaptionLimit(domain.ChannelTypeWebsite) {
		fields["body"] = ValidationBodyTooLong
	}
	var valid bool
	input.FileID, valid = common.NormalizeUUID(input.FileID)
	if !valid || input.ImageWidth < 0 || input.ImageHeight < 0 {
		fields["fileId"] = ValidationFileIDInvalid
	}
	return input, fields
}

// validWebsiteExternalID 校验网站访客规范化外部编号。
func validWebsiteExternalID(value string) bool {
	if len(value) != len(websiteExternalIDPrefix)+32 || !strings.HasPrefix(value, websiteExternalIDPrefix) {
		return false
	}
	for _, character := range value[len(websiteExternalIDPrefix):] {
		if (character < 'a' || character > 'f') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

// generateIDs 为一次渠道消息写入生成 UUIDv7。
func generateIDs() generatedIDs {
	values := make([]string, 7)
	for index := range values {
		values[index] = uuid.NewV7().String()
	}
	return generatedIDs{
		contact: values[0], channelIdentity: values[1], subject: values[2], conversation: values[3],
		participant: values[4], serviceSession: values[5], message: values[6],
	}
}

// loadWebsiteChannel 读取启用的网站渠道和路由配置。
func loadWebsiteChannel(ctx context.Context, db bun.IDB, channelID string) (*servermodels.Channel, error) {
	channel := &servermodels.Channel{}
	err := db.NewSelect().Model(channel).
		Where("c.id = ?", channelID).
		Where("c.type = ?", domain.ChannelTypeWebsite).
		Where("c.enabled = TRUE").
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrChannelNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load website channel: %w", err)
	}
	return channel, nil
}

// loadWebsiteVisitorIdentity 读取网站渠道内当前访客的渠道身份，尚未建立身份时返回 false；读操作不创建联系人。
func loadWebsiteVisitorIdentity(ctx context.Context, db bun.IDB, channel *servermodels.Channel, externalID string) (*servermodels.ContactChannelIdentity, bool, error) {
	identity := &servermodels.ContactChannelIdentity{}
	err := db.NewSelect().Model(identity).
		Where("cci.organization_id = ?", channel.OrganizationID).
		Where("cci.channel_id = ?", channel.ID).
		Where("cci.external_id = ?", externalID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("load website visitor identity: %w", err)
	}
	return identity, true, nil
}

// ensureContactSubject 取得或创建联系人聊天主体。
func ensureContactSubject(ctx context.Context, db bun.IDB, organizationID, contactID, subjectID string) (*servermodels.ChatSubject, error) {
	subject := &servermodels.ChatSubject{}
	err := db.NewSelect().Model(subject).
		Where("cs.organization_id = ?", organizationID).
		Where("cs.kind = ?", domain.ChatSubjectKindContact).
		Where("cs.source_id = ?", contactID).
		Scan(ctx)
	if err == nil {
		return subject, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("find contact chat subject: %w", err)
	}
	subject = &servermodels.ChatSubject{ID: subjectID, OrganizationID: organizationID, Kind: string(domain.ChatSubjectKindContact), SourceID: contactID}
	if _, err := db.NewInsert().Model(subject).
		Column("id", "organization_id", "kind", "source_id").
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("create contact chat subject: %w", err)
	}
	return subject, nil
}

// selectTargetConversation 取得指定客户线程或创建新的客户线程。
func selectTargetConversation(ctx context.Context, db bun.IDB, organizationID, channelIdentityID string, requestedConversationID *string, body, conversationID string) (*servermodels.Conversation, bool, error) {
	if requestedConversationID != nil {
		conversation := &servermodels.Conversation{}
		err := db.NewSelect().Model(conversation).
			Join("JOIN customer_conversations AS cc ON cc.organization_id = cv.organization_id AND cc.conversation_id = cv.id").
			Where("cv.organization_id = ?", organizationID).
			Where("cv.id = ?", *requestedConversationID).
			Where("cv.type = ?", domain.ConversationTypeCustomer).
			Where("cc.contact_channel_identity_id = ?", channelIdentityID).
			Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, ErrConversationNotFound
		}
		if err != nil {
			return nil, false, fmt.Errorf("load customer conversation: %w", err)
		}
		return conversation, false, nil
	}

	return createCustomerConversation(ctx, db, organizationID, channelIdentityID, body, conversationID)
}

// createCustomerConversation 创建客户线程及其渠道身份关系。
func createCustomerConversation(ctx context.Context, db bun.IDB, organizationID, channelIdentityID, body, conversationID string) (*servermodels.Conversation, bool, error) {
	// 从首条正文派生稳定会话标题。
	value := strings.Join(strings.Fields(body), " ")
	runes := []rune(value)
	if len(runes) > 60 {
		runes = runes[:60]
	}
	title := string(runes)
	conversation := &servermodels.Conversation{
		ID: conversationID, OrganizationID: organizationID, Type: string(domain.ConversationTypeCustomer),
		Status: string(domain.ConversationStatusActive), Title: &title,
	}
	if _, err := db.NewInsert().Model(conversation).
		Column("id", "organization_id", "type", "status", "title", "created_by_subject_id").
		Exec(ctx); err != nil {
		return nil, false, fmt.Errorf("create customer conversation: %w", err)
	}
	customer := &servermodels.CustomerConversation{ConversationID: conversation.ID, OrganizationID: organizationID, ContactChannelIdentityID: channelIdentityID}
	if _, err := db.NewInsert().Model(customer).
		Column("conversation_id", "organization_id", "contact_channel_identity_id").
		Exec(ctx); err != nil {
		return nil, false, fmt.Errorf("create customer conversation relation: %w", err)
	}
	return conversation, true, nil
}

// ensureContactParticipant 取得或恢复联系人参与者。
func ensureContactParticipant(ctx context.Context, db bun.IDB, organizationID, conversationID, subjectID, participantID string) (*servermodels.ConversationParticipant, error) {
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
				Set("updated_at = now()").
				WherePK().
				Where("organization_id = ?", organizationID).
				Exec(ctx); err != nil {
				return nil, fmt.Errorf("restore contact conversation participant: %w", err)
			}
			participant.LeftAt = nil
		}
		return participant, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("find contact conversation participant: %w", err)
	}
	participant = &servermodels.ConversationParticipant{
		ID: participantID, OrganizationID: organizationID, ConversationID: conversationID,
		SubjectID: subjectID, Role: string(domain.ConversationParticipantRoleMember),
	}
	if _, err := db.NewInsert().Model(participant).
		Column("id", "organization_id", "conversation_id", "subject_id", "role").
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("create contact conversation participant: %w", err)
	}
	return participant, nil
}

// selectServiceSession 选择线程当前批次或计算下一个批次序号。
func selectServiceSession(ctx context.Context, db bun.IDB, organizationID, conversationID, channelIdentityID string) (*servermodels.ServiceSession, bool, error) {
	session, err := chatstate.LockCurrentServiceSession(ctx, db, organizationID, conversationID)
	if err != nil {
		return nil, false, err
	}
	if session.ContactChannelIdentityID != channelIdentityID {
		return nil, false, ErrDataInvariant
	}
	switch domain.ServiceSessionStatus(session.Status) {
	case domain.ServiceSessionStatusOpen:
		return session, false, nil
	case domain.ServiceSessionStatusClosed:
		return &servermodels.ServiceSession{Sequence: session.Sequence + 1}, true, nil
	default:
		return nil, false, ErrDataInvariant
	}
}

// receiveWebsiteCustomerMessageResult 转换网站访客消息写入结果。
func receiveWebsiteCustomerMessageResult(organizationID string, received InboundCustomerMessageResult) ReceiveWebsiteCustomerMessageResult {
	var replyTo *MessageReference
	if reference := received.ReplyTo; reference != nil {
		replyTo = &MessageReference{ID: reference.ID, Deleted: reference.Deleted}
		if !reference.Deleted {
			replyTo.Author = domain.MessageAuthorAgent
			if reference.Sender.Kind == domain.ChatSubjectKindContact {
				replyTo.Author = domain.MessageAuthorVisitor
			}
			replyTo.Body = reference.Body
			replyTo.SenderIdentityType = reference.Sender.IdentityType
		}
	}
	return ReceiveWebsiteCustomerMessageResult{
		OrganizationID:          organizationID,
		Conversation:            received.Summary,
		CreatedConversation:     received.CreatedConversation,
		OpenedNewServiceSession: received.OpenedServiceSession,
		Message: Message{
			ClientMessageID: received.Message.ClientMessageID,
			ReplyTo:         replyTo,
			Attachment:      received.Attachment,
			ID:              received.Message.ID, Author: domain.MessageAuthorVisitor,
			MessageSeq: received.Message.MessageSeq, Body: received.Message.Body, OriginatedAt: received.Message.OriginatedAt,
			CreatedAt: received.Message.CreatedAt,
		},
	}
}

// loadConversationSummary 读取客户线程当前最后消息和当前客服周期摘要。
func loadConversationSummary(ctx context.Context, db bun.IDB, organizationID, conversationID, channelIdentityID string) (ConversationSummary, error) {
	row := conversationSummaryRow{}
	err := db.NewSelect().
		TableExpr("conversations AS cv").
		ColumnExpr("cv.id AS id").
		ColumnExpr("cv.title AS title").
		ColumnExpr("msg.originated_at AS last_message_at").
		ColumnExpr("msg.message_seq AS last_message_seq").
		ColumnExpr("? AS preview", messagequery.Summary("msg")).
		ColumnExpr("preview_oi.type AS preview_sender_identity_type").
		ColumnExpr("current.id AS service_session_id").
		ColumnExpr("current.status AS service_session_status").
		Join(`JOIN LATERAL (
 SELECT visible.* FROM messages AS visible
 WHERE visible.organization_id = cv.organization_id AND visible.conversation_id = cv.id AND visible.type IN (?, ?) AND visible.visibility = ? AND visible.deleted_at IS NULL
 ORDER BY visible.message_seq DESC LIMIT 1
 ) AS msg ON TRUE`, domain.MessageTypeText, domain.MessageTypeAttachment, domain.MessageVisibilityCustomerVisible).
		Join("LEFT JOIN conversation_participants AS preview_cp ON preview_cp.id = msg.sender_participant_id AND preview_cp.organization_id = msg.organization_id AND preview_cp.conversation_id = msg.conversation_id").
		Join("LEFT JOIN chat_subjects AS preview_cs ON preview_cs.id = preview_cp.subject_id AND preview_cs.organization_id = preview_cp.organization_id").
		Join("LEFT JOIN organization_identities AS preview_oi ON preview_oi.id = preview_cs.source_id AND preview_oi.organization_id = preview_cs.organization_id AND preview_cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("JOIN customer_conversations AS cc ON cc.organization_id = cv.organization_id AND cc.conversation_id = cv.id").
		Join("JOIN service_sessions AS current ON current.organization_id = cc.organization_id AND current.conversation_id = cc.conversation_id AND current.id = cc.current_service_session_id").
		Where("cv.organization_id = ?", organizationID).
		Where("cv.id = ?", conversationID).
		Where("cv.type = ?", domain.ConversationTypeCustomer).
		Where("cv.status IN (?, ?)", domain.ConversationStatusActive, domain.ConversationStatusArchived).
		Where("current.contact_channel_identity_id = ?", channelIdentityID).
		Scan(ctx, &row)
	if err != nil {
		return ConversationSummary{}, fmt.Errorf("load customer conversation summary: %w", err)
	}
	return conversationSummaryFromRow(row), nil
}

// retryableUniqueViolation 返回允许重试的并发唯一约束。
func retryableUniqueViolation(err error, constraintNames map[string]struct{}) (string, bool) {
	constraint, ok := pgerr.UniqueViolation(err)
	if !ok {
		return "", false
	}
	_, retryable := constraintNames[constraint]
	return constraint, retryable
}
