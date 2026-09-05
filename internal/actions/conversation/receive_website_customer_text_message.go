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

// ReceiveWebsiteCustomerTextMessageAction 持久化网站访客文本消息。
type ReceiveWebsiteCustomerTextMessageAction struct {
	db             *bun.DB
	agentScheduler CustomerAgentMessageScheduler
}

// CustomerAgentMessageScheduler 把网站客户消息加入当前 AI 客服的持久输入流。
type CustomerAgentMessageScheduler interface {
	ScheduleCustomerAuto(context.Context, bun.IDB, string, string, string, string) (bool, error)
}

// AgentMessageScheduler 同时调度内部单聊和网站客户 Agent 输入。
type AgentMessageScheduler interface {
	DirectAgentMessageScheduler
	CustomerAgentMessageScheduler
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

type routeSnapshot struct {
	teamID             *string
	assigneeIdentityID *string
	assignedAt         *time.Time
}

// NewReceiveWebsiteCustomerTextMessageAction 创建网站访客文本消息操作。
func NewReceiveWebsiteCustomerTextMessageAction(db *bun.DB, agentScheduler CustomerAgentMessageScheduler) *ReceiveWebsiteCustomerTextMessageAction {
	return &ReceiveWebsiteCustomerTextMessageAction{db: db, agentScheduler: agentScheduler}
}

// Execute 在一个可重试事务中写入网站访客文本消息。
func (a *ReceiveWebsiteCustomerTextMessageAction) Execute(ctx context.Context, input WebsiteCustomerTextMessageInput) (ReceiveWebsiteCustomerTextMessageResult, error) {
	normalized, fields := normalizeWebsiteMessageInput(input)
	if len(fields) > 0 {
		return ReceiveWebsiteCustomerTextMessageResult{}, &ValidationError{Fields: fields}
	}
	originatedAt := time.Now().UTC()
	idempotencyKey := "chmsg:" + normalized.ChannelID + ":" + normalized.ClientMessageID

	var err error
	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		var result ReceiveWebsiteCustomerTextMessageResult
		err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			var executeErr error
			result, executeErr = a.executeTransaction(ctx, tx, normalized, originatedAt, idempotencyKey)
			return executeErr
		})
		if err == nil {
			return result, nil
		}
		constraint, retryable := retryableUniqueViolation(err, websiteMessageRetryableConstraintNames)
		if !retryable {
			return ReceiveWebsiteCustomerTextMessageResult{}, err
		}
		if attempt < maxWriteAttempts-1 {
			slog.Info("网站访客消息写入重试", "channel_id", normalized.ChannelID, "attempt", attempt+2, "constraint", constraint)
		}
	}
	slog.Warn("网站访客消息写入重试耗尽", "channel_id", normalized.ChannelID, "error", err)
	return ReceiveWebsiteCustomerTextMessageResult{}, fmt.Errorf("receive website message retries exhausted: %w", err)
}

// executeTransaction 执行一次完整的网站访客消息事务。
func (a *ReceiveWebsiteCustomerTextMessageAction) executeTransaction(ctx context.Context, tx bun.Tx, input WebsiteCustomerTextMessageInput, originatedAt time.Time, idempotencyKey string) (ReceiveWebsiteCustomerTextMessageResult, error) {
	channel, err := loadWebsiteChannel(ctx, tx, input.ChannelID)
	if err != nil {
		return ReceiveWebsiteCustomerTextMessageResult{}, err
	}
	received, err := ReceiveInboundCustomerTextMessage(ctx, tx, channel, InboundCustomerTextMessageInput{
		ExternalID: input.ExternalID, RequestedConversationID: input.ConversationID,
		Body: input.Body, IdempotencyKey: idempotencyKey, OriginatedAt: originatedAt,
	})
	if err != nil {
		return ReceiveWebsiteCustomerTextMessageResult{}, err
	}
	if !received.Inserted {
		slog.Debug("网站访客消息幂等命中",
			"channel_id", channel.ID,
			"conversation_id", received.Message.ConversationID,
			"message_id", received.Message.ID,
		)
		return receiveWebsiteCustomerTextMessageResult(received), nil
	}
	if a.agentScheduler == nil {
		return ReceiveWebsiteCustomerTextMessageResult{}, errors.New("customer agent scheduler is unavailable")
	}
	if _, err := a.agentScheduler.ScheduleCustomerAuto(
		ctx, tx, channel.OrganizationID, received.Message.ConversationID, received.Session.ID, received.Message.ID,
	); err != nil {
		return ReceiveWebsiteCustomerTextMessageResult{}, fmt.Errorf("schedule website customer agent: %w", err)
	}
	return receiveWebsiteCustomerTextMessageResult(received), nil
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
	if !common.ValidUUID(input.ClientMessageID) {
		fields["clientMessageId"] = ValidationClientMessageIDInvalid
	}
	if input.Body == "" {
		fields["body"] = ValidationBodyRequired
	} else if utf8.RuneCountInString(input.Body) > 4000 {
		fields["body"] = ValidationBodyTooLong
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
	session, err := lockCurrentServiceSession(ctx, db, organizationID, conversationID)
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

// resolveRouteSnapshot 解析新客服处理周期的渠道路由快照。
func resolveRouteSnapshot(ctx context.Context, db bun.IDB, channel *servermodels.Channel, now time.Time) (routeSnapshot, error) {
	channelType := domain.ChannelType(channel.Type)
	if route, available, err := availableRoute(ctx, db, channel.OrganizationID, channelType, domain.ChannelRoutingTargetType(channel.InitialRoutingTargetType), channel.InitialRoutingTargetID, now); err != nil {
		return routeSnapshot{}, fmt.Errorf("resolve message channel initial route: %w", err)
	} else if available {
		return route, nil
	}
	slog.Warn("消息渠道初始路由不可用", "organization_id", channel.OrganizationID, "channel_id", channel.ID, "target_type", channel.InitialRoutingTargetType)
	if route, available, err := availableRoute(ctx, db, channel.OrganizationID, channelType, domain.ChannelRoutingTargetType(channel.FallbackRoutingTargetType), channel.FallbackRoutingTargetID, now); err != nil {
		return routeSnapshot{}, fmt.Errorf("resolve message channel fallback route: %w", err)
	} else if available {
		return route, nil
	}
	slog.Warn("消息渠道失败路由不可用，进入公共队列", "organization_id", channel.OrganizationID, "channel_id", channel.ID, "target_type", channel.FallbackRoutingTargetType)
	return routeSnapshot{}, nil
}

// availableRoute 判断路由目标当前是否可用。
func availableRoute(ctx context.Context, db bun.IDB, organizationID string, channelType domain.ChannelType, targetType domain.ChannelRoutingTargetType, targetID *string, now time.Time) (routeSnapshot, bool, error) {
	switch targetType {
	case domain.ChannelRoutingTargetTypePublicQueue:
		return routeSnapshot{}, true, nil
	case domain.ChannelRoutingTargetTypeTeam:
		if targetID == nil {
			return routeSnapshot{}, false, nil
		}
		available, err := db.NewSelect().Model((*servermodels.Team)(nil)).
			Where("organization_id = ?", organizationID).
			Where("id = ?", *targetID).
			Exists(ctx)
		return routeSnapshot{teamID: targetID}, available, err
	case domain.ChannelRoutingTargetTypeMember:
		if targetID == nil {
			return routeSnapshot{}, false, nil
		}
		identity, err := identityaction.LoadActiveCustomerServiceIdentity(ctx, db, organizationID, *targetID)
		if errors.Is(err, sql.ErrNoRows) {
			return routeSnapshot{}, false, nil
		}
		if err != nil {
			return routeSnapshot{}, false, err
		}
		if domain.OrganizationIdentityType(identity.Type) == domain.OrganizationIdentityTypeAgent && !domain.ChannelSupportsAgentAssignee(channelType) {
			slog.Warn("消息渠道不支持 AI 员工作为负责人",
				"organization_id", organizationID,
				"channel_type", channelType,
				"agent_identity_id", identity.ID,
			)
			return routeSnapshot{}, false, nil
		}
		return routeSnapshot{assigneeIdentityID: targetID, assignedAt: &now}, true, nil
	default:
		return routeSnapshot{}, false, nil
	}
}

// updateSessionSummary 按消息稳定顺序推进未结束批次摘要。
func updateSessionSummary(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession, message *servermodels.Message) error {
	_, err := db.NewUpdate().Model(session).
		Set("last_message_id = ?", message.ID).
		Set("last_message_at = ?", message.OriginatedAt).
		Set("last_message_source_order = ?", message.SourceOrder).
		Set("updated_at = now()").
		WherePK().
		Where("organization_id = ?", message.OrganizationID).
		Where("status = ?", domain.ServiceSessionStatusOpen).
		Where("(last_message_at, last_message_source_order, last_message_id) < (?, ?, ?)", message.OriginatedAt, message.SourceOrder, message.ID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update service session summary: %w", err)
	}
	return nil
}

// updateConversationSummary 按消息稳定顺序推进会话摘要。
func updateConversationSummary(ctx context.Context, db bun.IDB, conversation *servermodels.Conversation, message *servermodels.Message) error {
	query := db.NewUpdate().Model(conversation).
		Set("last_message_id = ?", message.ID).
		Set("last_message_at = ?", message.OriginatedAt).
		Set("last_message_source_order = ?", message.SourceOrder).
		Set("updated_at = now()").
		WherePK().
		Where("organization_id = ?", message.OrganizationID)
	if message.GroupMessageSequence != nil {
		query = query.Where("last_group_message_sequence = ?", *message.GroupMessageSequence)
	} else {
		query = query.Where("last_message_at IS NULL OR (last_message_at, last_message_source_order, last_message_id) < (?, ?, ?)", message.OriginatedAt, message.SourceOrder, message.ID)
	}
	_, err := query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("update conversation summary: %w", err)
	}
	return nil
}

// receiveWebsiteCustomerTextMessageResult 转换网站访客消息写入结果。
func receiveWebsiteCustomerTextMessageResult(received InboundCustomerTextMessageResult) ReceiveWebsiteCustomerTextMessageResult {
	return ReceiveWebsiteCustomerTextMessageResult{
		Conversation:            received.Summary,
		CreatedConversation:     received.CreatedConversation,
		OpenedNewServiceSession: received.OpenedServiceSession,
		Message: Message{
			ID: received.Message.ID, Author: domain.MessageAuthorVisitor,
			Body: received.Message.Body, OriginatedAt: received.Message.OriginatedAt,
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
		ColumnExpr("cv.last_message_at AS last_message_at").
		ColumnExpr("msg.body AS preview").
		ColumnExpr("current.id AS service_session_id").
		ColumnExpr("current.status AS service_session_status").
		Join("JOIN messages AS msg ON msg.id = cv.last_message_id AND msg.organization_id = cv.organization_id AND msg.conversation_id = cv.id AND msg.deleted_at IS NULL").
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
