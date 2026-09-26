//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	deliveryaction "github.com/runforyou-ai/cervi/internal/actions/customerdelivery"
	"github.com/runforyou-ai/cervi/internal/actions/customernotify"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/common/languagetag"
	"github.com/runforyou-ai/cervi/internal/common/searchtext"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

var memberMessageRetryableConstraintNames = map[string]struct{}{
	"chat_subjects_organization_kind_source_unique":             {},
	"conversation_participants_org_conversation_subject_unique": {},
	"messages_organization_idempotency_unique":                  {},
}

// SendServiceTextMessageAction 持久化企业成员的服务会话文本回复。
type SendServiceTextMessageAction struct {
	enqueuer servertask.TxEnqueuer
	db       *bun.DB
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
	ClientMessageID        *string                  `bun:"client_message_id"`
	ReplyToMessageID       *string                  `bun:"reply_to_message_id"`
	MessageSeq             int64                    `bun:"message_seq"`
	ID                     string                   `bun:"id"`
	CreatedAt              time.Time                `bun:"created_at"`
	ConversationID         string                   `bun:"conversation_id"`
	ServiceSessionID       *string                  `bun:"service_session_id"`
	SenderParticipantID    *string                  `bun:"sender_participant_id"`
	Type                   string                   `bun:"type"`
	Visibility             domain.MessageVisibility `bun:"visibility"`
	Body                   string                   `bun:"body"`
	Language               *string                  `bun:"language"`
	AuthoredBody           *string                  `bun:"authored_body"`
	AuthoredLanguage       *string                  `bun:"authored_language"`
	OriginatedAt           time.Time                `bun:"originated_at"`
	DeletedAt              *time.Time               `bun:"deleted_at"`
	SenderSubjectID        *string                  `bun:"sender_subject_id"`
	SenderSubjectKind      *string                  `bun:"sender_subject_kind"`
	SenderSubjectSourceID  *string                  `bun:"sender_subject_source_id"`
	JoinedServiceSessionID *string                  `bun:"joined_service_session_id"`
	AttachmentFileID       *string                  `bun:"attachment_file_id"`
	AttachmentName         *string                  `bun:"attachment_name"`
	AttachmentContentType  *string                  `bun:"attachment_content_type"`
	AttachmentByteSize     *int64                   `bun:"attachment_byte_size"`
	AttachmentImageWidth   *int                     `bun:"attachment_image_width"`
	AttachmentImageHeight  *int                     `bun:"attachment_image_height"`
	AttachmentTransfer     *string                  `bun:"attachment_transfer_status"`
}

// memberMessageExpectation 定义幂等命中时必须完全一致的成员发送意图。
type memberMessageExpectation struct {
	Attachment            *attachmentExpectation
	ConversationID        string
	Body                  string
	ReplyToMessageID      string
	Type                  domain.MessageType
	Visibility            domain.MessageVisibility
	RequireServiceSession bool
	// Translated 表示翻译发送，此时 Body 与保存的客服原话核对。
	Translated bool
}

// attachmentExpectation 定义附件消息幂等核对所需的文件事实。
type attachmentExpectation struct {
	FileID      string
	ImageWidth  int
	ImageHeight int
}

// customerMessagePayload 定义一次成员客户会话发送的消息内容。
type customerMessagePayload struct {
	Attachment       *customerAttachmentPayload
	ConversationID   string
	ClientMessageID  string
	Body             string
	ReplyToMessageID string
	Type             domain.MessageType
	Visibility       domain.MessageVisibility
	// MentionIdentityIDs 是内部备注提醒的企业成员身份。
	MentionIdentityIDs []string
	// Translation 是翻译发送时发给客户的译文，Body 为客服书写的原文。
	Translation *OutgoingTranslation
}

// customerAttachmentPayload 定义附件消息待关联的上传文件。
type customerAttachmentPayload struct {
	FileID      string
	ImageWidth  int
	ImageHeight int
}

// expectation 返回该次发送对应的幂等核对意图。
func (p customerMessagePayload) expectation() memberMessageExpectation {
	result := memberMessageExpectation{
		ConversationID: p.ConversationID, Body: p.Body, ReplyToMessageID: p.ReplyToMessageID,
		Type: p.Type, Visibility: p.Visibility, RequireServiceSession: true,
	}
	result.Translated = p.Translation != nil
	if p.Attachment != nil {
		result.Attachment = &attachmentExpectation{FileID: p.Attachment.FileID, ImageWidth: p.Attachment.ImageWidth, ImageHeight: p.Attachment.ImageHeight}
	}
	return result
}

// internalTextExpectation 构造内部会话文本消息的幂等核对意图。
func internalTextExpectation(conversationID, body, replyToMessageID string) memberMessageExpectation {
	return memberMessageExpectation{
		ConversationID: conversationID, Body: body, ReplyToMessageID: replyToMessageID,
		Type: domain.MessageTypeText, Visibility: domain.MessageVisibilityShared,
	}
}

// NewSendServiceTextMessageAction 创建成员服务会话回复操作。
func NewSendServiceTextMessageAction(db *bun.DB, enqueuer servertask.TxEnqueuer) *SendServiceTextMessageAction {
	return &SendServiceTextMessageAction{db: db, enqueuer: enqueuer}
}

// Execute 在一个可重试事务中写入成员客户会话回复。
func (a *SendServiceTextMessageAction) Execute(ctx context.Context, identity *servermodels.Identity, input ServiceTextMessageInput) (ConversationMessage, error) {
	normalized, fields := normalizeServiceTextMessageInput(input)
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
	idempotencyKey := "mmsg:" + identity.OrganizationIdentity.ID + ":" + normalized.ClientMessageID
	payload := customerMessagePayload{
		ConversationID: normalized.ConversationID, ClientMessageID: normalized.ClientMessageID,
		Body: normalized.Body, ReplyToMessageID: normalized.ReplyToMessageID, Type: domain.MessageTypeText,
		Visibility: normalized.Visibility, MentionIdentityIDs: normalized.MentionIdentityIDs, Translation: normalized.Translation,
	}

	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		var result ConversationMessage
		err = realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
			var executeErr error
			result, executeErr = sendCustomerMessage(ctx, tx, identity, a.enqueuer, payload, ids, idempotencyKey)
			return executeErr
		})
		if err == nil {
			// 发送结果与历史查询使用同一引用能力判定，未取得平台回执时不可被引用。
			var row conversationMessageRow
			if err := conversationMessagesQuery(a.db, identity, normalized.ConversationID).Where("msg.id = ?", result.ID).Scan(ctx, &row); err != nil {
				return ConversationMessage{}, fmt.Errorf("load sent message reference state: %w", err)
			}
			result.ReplyUnavailable = row.ReplyUnavailable
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

// SavedTranslation 返回本人以该发送编号已保存的翻译发送的译文与原话语言，未保存或未翻译时返回 nil；重试发送据此沿用首次发出的译文，与当前语言设置无关。
func (a *SendServiceTextMessageAction) SavedTranslation(ctx context.Context, identity *servermodels.Identity, clientMessageID string) (*OutgoingTranslation, error) {
	clientMessageID, valid := common.NormalizeUUID(clientMessageID)
	if !valid {
		return nil, nil
	}
	var rows []struct {
		Language       string `bun:"language"`
		Body           string `bun:"body"`
		SourceLanguage string `bun:"source_language"`
	}
	if err := a.db.NewSelect().
		TableExpr("messages AS msg").
		ColumnExpr("msg.language, msg.body, mt.language AS source_language").
		Join("JOIN message_translations AS mt ON mt.message_id = msg.id AND mt.organization_id = msg.organization_id AND mt.authored").
		Where("msg.organization_id = ? AND msg.idempotency_key = ? AND msg.language IS NOT NULL", identity.Organization.ID, "mmsg:"+identity.OrganizationIdentity.ID+":"+clientMessageID).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("load saved outgoing translation: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &OutgoingTranslation{Language: rows[0].Language, SourceLanguage: rows[0].SourceLanguage, Body: rows[0].Body}, nil
}

// sendCustomerMessage 执行一次完整的成员客户会话回复事务，文本与附件共用客服周期、引用和外发语义。
func sendCustomerMessage(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, enqueuer servertask.TxEnqueuer, input customerMessagePayload, ids memberMessageIDs, idempotencyKey string) (ConversationMessage, error) {
	// 内部备注不要求接待资格，对客回复只有开启接待的成员可以发送。
	internalNote := input.Visibility == domain.MessageVisibilityInternal
	lock := identityaction.LockActiveUser
	if !internalNote {
		lock = lockActiveCustomerHandler
	}
	if err := lock(ctx, tx, identity); err != nil {
		return ConversationMessage{}, err
	}
	service, err := chatstate.LoadServiceConversation(ctx, tx, identity.Organization.ID, input.ConversationID)
	if err != nil {
		return ConversationMessage{}, err
	}
	if err := rejectServiceRequester(ctx, tx, service, identity.OrganizationIdentity.ID); err != nil {
		return ConversationMessage{}, err
	}
	// 渠道投递路由只用于渠道来源的对客消息，其他来源的发起人直接在会话中读到回复。
	channelReply := !internalNote && domain.ServiceSource(service.Source) == domain.ServiceSourceChannel
	var route deliveryaction.Route
	if channelReply {
		route, err = deliveryaction.Prepare(ctx, tx, identity.Organization.ID, input.ConversationID)
		if errors.Is(err, deliveryaction.ErrUnavailable) {
			return ConversationMessage{}, ErrConversationNotFound
		}
		if err != nil {
			return ConversationMessage{}, err
		}
	}
	conversation, err := chatstate.LockServiceConversation(ctx, tx, identity.Organization.ID, input.ConversationID)
	if err != nil {
		return ConversationMessage{}, err
	}
	if channelReply && route.ChannelType != domain.ChannelTypeWebsite && route.ChannelType != domain.ChannelTypeTelegram {
		return ConversationMessage{}, &ConflictError{Reason: ConflictReasonChannelOutboundUnsupported}
	}
	session, err := chatstate.LockCurrentServiceSession(ctx, tx, identity.Organization.ID, conversation.ID)
	if err != nil {
		return ConversationMessage{}, err
	}
	if saved, found, err := loadIdempotentCustomerMessage(ctx, tx, identity, input, idempotencyKey); err != nil || found {
		return saved, err
	}
	// 译文按来源渠道的文本上限校验。
	if channelReply && input.Translation != nil && utf8.RuneCountInString(input.Translation.Body) > domain.ChannelTextLimit(route.ChannelType) {
		return ConversationMessage{}, &ConflictError{Reason: ConflictReasonTranslationTooLong}
	}
	// 渠道来源的附件按来源渠道的外发能力、字节上限和说明上限校验。
	if channelReply && input.Attachment != nil {
		if !domain.ChannelSupportsOutboundAttachment(route.ChannelType) {
			return ConversationMessage{}, &ConflictError{Reason: ConflictReasonChannelAttachmentUnsupported}
		}
		if utf8.RuneCountInString(input.Body) > domain.ChannelCaptionLimit(route.ChannelType) {
			return ConversationMessage{}, &ConflictError{Reason: ConflictReasonCaptionTooLong}
		}
	}

	if channelReply && route.ChannelType == domain.ChannelTypeTelegram {
		if !route.Enabled || route.BotID == nil {
			return ConversationMessage{}, &ConflictError{Reason: ConflictReasonChannelOutboundUnavailable}
		}
		if input.ReplyToMessageID != "" {
			// 引用必须具有当前机器人、同一聊天内的确定平台消息身份。
			var providerID string
			err := tx.NewSelect().TableExpr("channel_messages AS cm").ColumnExpr("cm.provider_message_id").
				Join("JOIN contact_channel_identities AS cci ON cci.organization_id = cm.organization_id AND cci.channel_id = cm.channel_id AND cci.external_id = cm.provider_conversation_id").
				Join("JOIN messages AS msg ON msg.id = cm.message_id AND msg.organization_id = cm.organization_id AND msg.conversation_id = cm.conversation_id").
				Where("cm.organization_id = ? AND cm.conversation_id = ? AND cm.message_id = ?", identity.Organization.ID, conversation.ID, input.ReplyToMessageID).
				Where("cm.channel_id = ? AND cm.provider_account_id = ? AND cci.id = ?", route.ChannelID, strconv.FormatInt(*route.BotID, 10), route.IdentityID).
				Where("msg.type IN (?) AND msg.deleted_at IS NULL", bun.In([]domain.MessageType{domain.MessageTypeText, domain.MessageTypeAttachment})).Scan(ctx, &providerID)
			if errors.Is(err, sql.ErrNoRows) {
				return ConversationMessage{}, &ConflictError{Reason: ConflictReasonReplyTargetInvalid}
			}
			if err != nil {
				return ConversationMessage{}, err
			}
			route.ReplyProviderMessageID = &providerID
		}
	}
	// 取得客服周期锁后生成消息时间。
	originatedAt := time.Now().UTC()
	// 计算成员回复对应的客服周期状态迁移。
	status := domain.ServiceSessionStatus(session.Status)
	if status != domain.ServiceSessionStatusOpen && status != domain.ServiceSessionStatusClosed {
		return ConversationMessage{}, ErrDataInvariant
	}
	// 对客回复要求周期开放，内部备注可以补记到已关闭周期。
	if !internalNote && status == domain.ServiceSessionStatusClosed {
		return ConversationMessage{}, &ConflictError{Reason: ConflictReasonServiceSessionNotReplyable}
	}
	// 对客回复要求周期无人负责或由本人负责，内部备注对能读取该会话的成员开放。
	if !internalNote && session.AssigneeIdentityID != nil && *session.AssigneeIdentityID != identity.OrganizationIdentity.ID {
		return ConversationMessage{}, &ConflictError{Reason: ConflictReasonServiceSessionOwned}
	}
	replyTo, err := loadConversationReplyTarget(ctx, tx, identity.Organization.ID, conversation.ID, input.ReplyToMessageID)
	if err != nil {
		return ConversationMessage{}, err
	}
	// 对客消息只能引用对客可见的消息。
	if !internalNote && replyTo != nil && replyTo.Visibility == domain.MessageVisibilityInternal {
		return ConversationMessage{}, &ConflictError{Reason: ConflictReasonReplyTargetInvalid}
	}
	if !internalNote {
		plan := memberReplySessionPlan{assign: session.AssigneeIdentityID == nil}
		if err := applyMemberReplySessionPlan(ctx, tx, session, identity.OrganizationIdentity.ID, originatedAt, plan); err != nil {
			return ConversationMessage{}, err
		}
		// 回复无人负责的周期即领取该周期。
		if plan.assign {
			if err := appendServiceSessionEvent(ctx, tx, identity, conversation, session, domain.ConversationSystemEventServiceSessionClaimed, nil, nil); err != nil {
				return ConversationMessage{}, err
			}
		}
	}
	// 取得或创建发送者与提醒成员的聊天主体，再建立发送者参与者和协作者关系。
	subject, mentions, err := ensureNoteSubjects(ctx, tx, identity, ids.subject, input.MentionIdentityIDs)
	if err != nil {
		return ConversationMessage{}, err
	}
	// 内部备注对发起人不可见，不能提醒发起人。
	for _, mention := range mentions {
		if mention.ChatSubjectID == service.RequesterSubjectID {
			return ConversationMessage{}, &ConflictError{Reason: ConflictReasonNoteMentionTargetInvalid}
		}
	}
	participant, err := ensureMemberConversationParticipant(ctx, tx, identity.Organization.ID, conversation.ID, subject.ID, ids.participant)
	if err != nil {
		return ConversationMessage{}, err
	}
	for _, mention := range mentions {
		if _, err := ensureMemberConversationParticipant(ctx, tx, identity.Organization.ID, conversation.ID, mention.ChatSubjectID, uuid.NewV7().String()); err != nil {
			return ConversationMessage{}, err
		}
	}

	message := &servermodels.Message{
		ID: ids.message, OrganizationID: identity.Organization.ID, ConversationID: conversation.ID,
		ServiceSessionID: &session.ID, SenderParticipantID: &participant.ID,
		Type: string(input.Type), Visibility: string(input.Visibility), Body: input.Body, ClientMessageID: &input.ClientMessageID, IdempotencyKey: &idempotencyKey, OriginatedAt: originatedAt,
	}
	// 翻译发送时客户收到译文，原文与译文都进入检索。
	if input.Translation != nil {
		message.Body, message.Language = input.Translation.Body, &input.Translation.Language
		message.SearchVector = searchtext.Vector(input.Translation.Body, input.Body)
	}
	if replyTo != nil {
		message.ReplyToMessageID = &replyTo.ID
	}
	var attachment *MessageAttachment
	if input.Attachment != nil {
		attachment, err = lockCustomerAttachmentFile(ctx, tx, identity, route.ChannelType, *input.Attachment)
		if err != nil {
			return ConversationMessage{}, err
		}
		message.SearchVector = searchtext.Vector(input.Body, attachment.Name)
	}
	message, inserted, err := chatstate.AppendMessage(ctx, tx, conversation, message)
	if err != nil {
		return ConversationMessage{}, err
	}
	if !inserted {
		saved, _, err := loadIdempotentCustomerMessage(ctx, tx, identity, input, idempotencyKey)
		return saved, err
	}
	if err := createMessageMentions(ctx, tx, identity.Organization.ID, message.ID, mentions); err != nil {
		return ConversationMessage{}, err
	}
	if attachment != nil {
		if err := saveCustomerAttachment(ctx, tx, identity.Organization.ID, message.ID, *attachment); err != nil {
			return ConversationMessage{}, err
		}
	}
	// 客服书写的原文按客服语言保存为该消息的译文。
	if input.Translation != nil {
		if _, err := tx.NewInsert().Model(&servermodels.MessageTranslation{
			MessageID: message.ID, Language: input.Translation.SourceLanguage, OrganizationID: identity.Organization.ID, Body: input.Body, Authored: true,
		}).Exec(ctx); err != nil {
			return ConversationMessage{}, fmt.Errorf("save authored message translation: %w", err)
		}
	}
	if !internalNote {
		if route.ChannelType == domain.ChannelTypeTelegram {
			if err := deliveryaction.Enqueue(ctx, tx, enqueuer, route, message); err != nil {
				return ConversationMessage{}, err
			}
		}
		// 网站访客未读到真人回复时由邮件通知。
		if route.ChannelType == domain.ChannelTypeWebsite {
			if err := customernotify.ScheduleCheck(ctx, tx, identity.Organization.ID, conversation.ID, originatedAt); err != nil {
				return ConversationMessage{}, err
			}
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
	}
	result := memberConversationMessage(message, subject.ID, identity.OrganizationIdentity)
	result.ReplyTo = replyTo
	result.Attachment = attachment
	result.Mentions = mentions
	if input.Translation != nil {
		result.Translation = &MessageTranslation{Language: input.Translation.SourceLanguage, Body: input.Body}
	}
	return result, nil
}

// ensureNoteSubjects 校验内部备注提醒的企业成员，按身份编号顺序取得或创建发送者与提醒成员的聊天主体，提醒按正文顺序返回。
func ensureNoteSubjects(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, senderSubjectID string, identityIDs []string) (*servermodels.ChatSubject, []ConversationMessageMention, error) {
	var rows []struct {
		ID          string `bun:"id"`
		DisplayName string `bun:"display_name"`
	}
	if len(identityIDs) > 0 {
		if err := tx.NewSelect().TableExpr("organization_identities AS oi").
			ColumnExpr("oi.id, oi.display_name").
			Join("JOIN users AS u ON u.organization_id = oi.organization_id AND u.identity_id = oi.id").
			Where("oi.organization_id = ? AND oi.id IN (?)", identity.Organization.ID, bun.In(identityIDs)).
			Where("oi.type = ? AND u.status = ?", domain.OrganizationIdentityTypeUser, domain.UserStatusActive).
			Where("oi.id <> ?", identity.OrganizationIdentity.ID).
			Scan(ctx, &rows); err != nil {
			return nil, nil, fmt.Errorf("load note mention targets: %w", err)
		}
		if len(rows) != len(identityIDs) {
			return nil, nil, &ConflictError{Reason: ConflictReasonNoteMentionTargetInvalid}
		}
	}
	names := make(map[string]string, len(rows))
	for _, row := range rows {
		names[row.ID] = row.DisplayName
	}
	// 并发互相提醒的事务按同一身份编号顺序创建聊天主体，唯一约束等待不会形成循环。
	newSubjectIDs := map[string]string{identity.OrganizationIdentity.ID: senderSubjectID}
	for _, identityID := range identityIDs {
		newSubjectIDs[identityID] = uuid.NewV7().String()
	}
	subjects := make(map[string]*servermodels.ChatSubject, len(newSubjectIDs))
	for _, identityID := range slices.Sorted(maps.Keys(newSubjectIDs)) {
		subject, err := chatstate.EnsureOrganizationIdentityChatSubject(ctx, tx, identity.Organization.ID, identityID, newSubjectIDs[identityID])
		if err != nil {
			return nil, nil, err
		}
		subjects[identityID] = subject
	}
	mentions := make([]ConversationMessageMention, 0, len(identityIDs))
	for _, identityID := range identityIDs {
		name := names[identityID]
		mentions = append(mentions, ConversationMessageMention{
			ChatSubjectID: subjects[identityID].ID, Kind: domain.ChatSubjectKindOrganizationIdentity,
			SourceID: identityID, DisplayName: &name, IdentityType: domain.OrganizationIdentityTypeUser,
		})
	}
	return subjects[identity.OrganizationIdentity.ID], mentions, nil
}

// loadIdempotentCustomerMessage 校验成员客户消息的完整发送意图，包括内部备注提醒的成员。
func loadIdempotentCustomerMessage(ctx context.Context, db bun.IDB, identity *servermodels.Identity, input customerMessagePayload, idempotencyKey string) (ConversationMessage, bool, error) {
	saved, found, err := loadIdempotentMemberMessage(ctx, db, identity, input.expectation(), idempotencyKey)
	if err != nil || !found {
		return saved, found, err
	}
	saved.Mentions, err = loadPersistedMessageMentions(ctx, db, identity.Organization.ID, saved.ID)
	if err != nil {
		return ConversationMessage{}, true, err
	}
	stored := make([]string, 0, len(saved.Mentions))
	for _, mention := range saved.Mentions {
		stored = append(stored, mention.SourceID)
	}
	sent := slices.Clone(input.MentionIdentityIDs)
	slices.Sort(stored)
	slices.Sort(sent)
	if !slices.Equal(stored, sent) {
		return ConversationMessage{}, true, &ConflictError{Reason: ConflictReasonIdempotencyMismatch}
	}
	return saved, true, nil
}

// normalizeServiceTextMessageInput 规范化并校验成员客户消息输入。
func normalizeServiceTextMessageInput(input ServiceTextMessageInput) (ServiceTextMessageInput, map[string]ValidationCode) {
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
	if input.ReplyToMessageID != "" {
		input.ReplyToMessageID, valid = common.NormalizeUUID(input.ReplyToMessageID)
		if !valid {
			fields["replyToMessageId"] = ValidationReplyToMessageIDInvalid
		}
	}
	if input.Body == "" {
		fields["body"] = ValidationBodyRequired
	} else if utf8.RuneCountInString(input.Body) > 4000 {
		fields["body"] = ValidationBodyTooLong
	}
	if input.Visibility == "" {
		input.Visibility = domain.MessageVisibilityShared
	}
	if input.Visibility != domain.MessageVisibilityShared && input.Visibility != domain.MessageVisibilityInternal {
		fields["visibility"] = ValidationMessageVisibilityInvalid
	}
	// 只有内部备注可以提醒企业成员，同一成员只提醒一次。
	if len(input.MentionIdentityIDs) > 0 && input.Visibility != domain.MessageVisibilityInternal {
		fields["mentionIdentityIds"] = ValidationMentionIdentityIDsInvalid
	}
	mentionIdentityIDs := make([]string, 0, len(input.MentionIdentityIDs))
	for _, identityID := range input.MentionIdentityIDs {
		normalized, valid := common.NormalizeUUID(identityID)
		if !valid || slices.Contains(mentionIdentityIDs, normalized) {
			fields["mentionIdentityIds"] = ValidationMentionIdentityIDsInvalid
			continue
		}
		mentionIdentityIDs = append(mentionIdentityIDs, normalized)
	}
	input.MentionIdentityIDs = mentionIdentityIDs
	// 译文只用于对客回复，语言标签取规范形式。
	if input.Translation != nil {
		translation := *input.Translation
		translation.Body = strings.TrimSpace(translation.Body)
		language, languageValid := languagetag.Normalize(translation.Language)
		source, sourceValid := languagetag.Normalize(translation.SourceLanguage)
		if input.Visibility != domain.MessageVisibilityShared || !languageValid || !sourceValid || translation.Body == "" || utf8.RuneCountInString(translation.Body) > 8000 {
			fields["translation"] = ValidationTranslationInvalid
		}
		translation.Language, translation.SourceLanguage = language, source
		input.Translation = &translation
	}
	return input, fields
}

// loadIdempotentMemberMessage 校验并返回已经保存的成员消息。
func loadIdempotentMemberMessage(ctx context.Context, db bun.IDB, identity *servermodels.Identity, expectation memberMessageExpectation, idempotencyKey string) (ConversationMessage, bool, error) {
	row := idempotentMemberMessageRow{}
	err := db.NewSelect().
		TableExpr("messages AS msg").
		ColumnExpr("msg.id AS id").
		ColumnExpr("msg.client_message_id").
		ColumnExpr("msg.created_at AS created_at").
		ColumnExpr("msg.conversation_id AS conversation_id").
		ColumnExpr("msg.service_session_id AS service_session_id").
		ColumnExpr("msg.sender_participant_id AS sender_participant_id").
		ColumnExpr("msg.type AS type").
		ColumnExpr("msg.visibility AS visibility").
		ColumnExpr("msg.body AS body").
		ColumnExpr("msg.language AS language").
		ColumnExpr("authored.body AS authored_body, authored.language AS authored_language").
		ColumnExpr("msg.reply_to_message_id AS reply_to_message_id").
		ColumnExpr("msg.originated_at AS originated_at").
		ColumnExpr("msg.deleted_at AS deleted_at").
		ColumnExpr("msg.message_seq AS message_seq").
		ColumnExpr("cs.id AS sender_subject_id").
		ColumnExpr("cs.kind AS sender_subject_kind").
		ColumnExpr("cs.source_id AS sender_subject_source_id").
		ColumnExpr("ss.id AS joined_service_session_id").
		ColumnExpr("ma.file_id::text AS attachment_file_id").
		ColumnExpr("ma.name AS attachment_name").
		ColumnExpr("ma.content_type AS attachment_content_type").
		ColumnExpr("ma.byte_size AS attachment_byte_size").
		ColumnExpr("ma.image_width AS attachment_image_width").
		ColumnExpr("ma.image_height AS attachment_image_height").
		ColumnExpr("ma.transfer_status AS attachment_transfer_status").
		Join("LEFT JOIN message_attachments AS ma ON ma.message_id = msg.id AND ma.organization_id = msg.organization_id").
		Join("LEFT JOIN message_translations AS authored ON authored.message_id = msg.id AND authored.organization_id = msg.organization_id AND authored.authored").
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
	storedReply := ""
	if row.ReplyToMessageID != nil {
		storedReply = *row.ReplyToMessageID
	}
	serviceSessionMatches := row.ServiceSessionID == nil && row.JoinedServiceSessionID == nil
	if expectation.RequireServiceSession {
		serviceSessionMatches = row.ServiceSessionID != nil && row.JoinedServiceSessionID != nil && *row.ServiceSessionID == *row.JoinedServiceSessionID
	}
	// 附件消息额外核对文件与图片尺寸，文本消息不得关联附件。
	attachmentMatches := row.AttachmentFileID == nil
	if expectation.Attachment != nil {
		attachmentMatches = row.AttachmentFileID != nil && *row.AttachmentFileID == expectation.Attachment.FileID &&
			row.AttachmentImageWidth != nil && *row.AttachmentImageWidth == expectation.Attachment.ImageWidth &&
			row.AttachmentImageHeight != nil && *row.AttachmentImageHeight == expectation.Attachment.ImageHeight
	}
	// 翻译发送核对客服书写的原话，译文每次生成可能不同。
	bodyMatches := row.Body == expectation.Body && row.AuthoredBody == nil
	if expectation.Translated {
		bodyMatches = row.AuthoredBody != nil && *row.AuthoredBody == expectation.Body
	}
	messageMatches := storedReply == expectation.ReplyToMessageID && row.ConversationID == expectation.ConversationID && bodyMatches &&
		row.Type == string(expectation.Type) && row.Visibility == expectation.Visibility && row.DeletedAt == nil &&
		serviceSessionMatches && attachmentMatches &&
		row.SenderParticipantID != nil && row.SenderSubjectID != nil && row.SenderSubjectKind != nil && row.SenderSubjectSourceID != nil &&
		*row.SenderSubjectKind == string(domain.ChatSubjectKindOrganizationIdentity) && *row.SenderSubjectSourceID == identity.OrganizationIdentity.ID
	if !messageMatches {
		return ConversationMessage{}, true, &ConflictError{Reason: ConflictReasonIdempotencyMismatch}
	}
	message := &servermodels.Message{
		ClientMessageID: row.ClientMessageID, ID: row.ID, CreatedAt: row.CreatedAt, ConversationID: row.ConversationID,
		ServiceSessionID: row.ServiceSessionID, SenderParticipantID: row.SenderParticipantID,
		Type: row.Type, Visibility: string(row.Visibility), Body: row.Body, Language: row.Language, OriginatedAt: row.OriginatedAt, DeletedAt: row.DeletedAt, MessageSeq: row.MessageSeq,
	}
	result := memberConversationMessage(message, *row.SenderSubjectID, identity.OrganizationIdentity)
	if row.AuthoredBody != nil && row.AuthoredLanguage != nil {
		result.Translation = &MessageTranslation{Language: *row.AuthoredLanguage, Body: *row.AuthoredBody}
	}
	// 附件行存在时其余列均非空。
	if row.AttachmentFileID != nil {
		result.Attachment = &MessageAttachment{
			ID: *row.AttachmentFileID, Name: *row.AttachmentName, ContentType: *row.AttachmentContentType,
			ByteSize: *row.AttachmentByteSize, ImageWidth: *row.AttachmentImageWidth, ImageHeight: *row.AttachmentImageHeight,
			TransferStatus: domain.MessageAttachmentTransferStatus(*row.AttachmentTransfer),
		}
	}
	if storedReply != "" {
		result.ReplyTo, err = loadMessageReference(ctx, db, identity.Organization.ID, expectation.ConversationID, storedReply)
		if err != nil {
			return ConversationMessage{}, true, fmt.Errorf("load idempotent message reference: %w", err)
		}
	}
	return result, true, nil
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
			Set("assigned_at = COALESCE(assigned_at, ?)", now).
			Set("assignee_assigned_at = ?", now).
			Set("queued_at = NULL").
			Set("reminded_at = NULL")
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
func memberConversationMessage(message *servermodels.Message, subjectID string, identity servermodels.OrganizationIdentity) ConversationMessage {
	name := identity.DisplayName
	identityType := domain.OrganizationIdentityType(identity.Type)
	return ConversationMessage{
		ClientMessageID: message.ClientMessageID, ID: message.ID, Type: domain.MessageType(message.Type), Visibility: domain.MessageVisibility(message.Visibility), Body: message.Body, Language: message.Language,
		OriginatedAt: message.OriginatedAt, SourceOrder: message.SourceOrder, CreatedAt: message.CreatedAt, MentionAll: message.MentionAll, MessageSeq: message.MessageSeq,
		Sender: &ConversationMessageSender{
			ChatSubjectID: subjectID, Kind: domain.ChatSubjectKindOrganizationIdentity,
			SourceID: identity.ID, DisplayName: &name, AvatarFileID: identity.AvatarFileID, IdentityType: &identityType,
		},
	}
}
