//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
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

var directMessageRetryableConstraintNames = map[string]struct{}{
	"messages_organization_idempotency_unique": {},
}

// SendFirstDirectTextMessageAction 发送首条单聊消息并按需创建长期会话。
type SendFirstDirectTextMessageAction struct {
	db             *bun.DB
	agentScheduler DirectAgentMessageScheduler
}

// FindDirectConversationQuery 按目标身份查找当前成员的活跃长期单聊。
type FindDirectConversationQuery struct {
	db *bun.DB
}

// SendDirectTextMessageAction 持久化企业成员内部单聊文本消息。
type SendDirectTextMessageAction struct {
	db             *bun.DB
	agentScheduler DirectAgentMessageScheduler
}

// DirectAgentMessageScheduler 把发给 Agent 的单聊消息加入持久化输入流。
type DirectAgentMessageScheduler interface {
	Schedule(context.Context, bun.IDB, string, string, string, string, string) error
}

type directTargetRow struct {
	IdentityID   string                          `bun:"identity_id"`
	IdentityType domain.OrganizationIdentityType `bun:"identity_type"`
	DisplayName  string                          `bun:"display_name"`
}

type directConversationIDs struct {
	conversation       string
	currentSubject     string
	targetSubject      string
	currentParticipant string
	targetParticipant  string
}

type directConversationSummaryRow struct {
	ID            string     `bun:"id"`
	Preview       *string    `bun:"preview"`
	LastMessageAt *time.Time `bun:"last_message_at"`
}

type directSendContextRow struct {
	ConversationID string                          `bun:"conversation_id"`
	ParticipantID  string                          `bun:"participant_id"`
	SubjectID      string                          `bun:"subject_id"`
	PeerType       domain.OrganizationIdentityType `bun:"peer_type"`
	PeerIdentityID string                          `bun:"peer_identity_id"`
	PeerRevisionID *string                         `bun:"peer_revision_id"`
}

// NewSendFirstDirectTextMessageAction 创建首条单聊消息发送操作。
func NewSendFirstDirectTextMessageAction(db *bun.DB, agentScheduler DirectAgentMessageScheduler) *SendFirstDirectTextMessageAction {
	return &SendFirstDirectTextMessageAction{db: db, agentScheduler: agentScheduler}
}

// NewFindDirectConversationQuery 创建内部单聊查找查询。
func NewFindDirectConversationQuery(db *bun.DB) *FindDirectConversationQuery {
	return &FindDirectConversationQuery{db: db}
}

// Execute 返回当前成员与目标身份的活跃长期单聊。
func (q *FindDirectConversationQuery) Execute(ctx context.Context, identity *servermodels.Identity, targetIdentityID string) (*DirectConversationSummary, error) {
	targetIdentityID, valid := common.NormalizeUUID(targetIdentityID)
	if !valid || targetIdentityID == identity.OrganizationIdentity.ID {
		return nil, ErrDirectTargetNotFound
	}
	target, err := loadDirectTarget(ctx, q.db, identity.Organization.ID, targetIdentityID)
	if err != nil {
		return nil, err
	}
	conversation, err := findDirectConversation(ctx, q.db, identity.Organization.ID, identity.OrganizationIdentity.ID, targetIdentityID)
	if err != nil || conversation == nil || conversation.Status != string(domain.ConversationStatusActive) {
		return nil, err
	}
	summary, err := loadDirectConversationSummary(ctx, q.db, identity.Organization.ID, conversation.ID, target)
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

// NewSendDirectTextMessageAction 创建内部单聊发送操作。
func NewSendDirectTextMessageAction(db *bun.DB, agentScheduler DirectAgentMessageScheduler) *SendDirectTextMessageAction {
	return &SendDirectTextMessageAction{db: db, agentScheduler: agentScheduler}
}

// Execute 发送首条单聊消息并按需创建当前成员与目标成员的长期单聊。
func (a *SendFirstDirectTextMessageAction) Execute(ctx context.Context, identity *servermodels.Identity, input FirstDirectTextMessageInput) (FirstDirectTextMessageResult, error) {
	targetIdentityID, valid := common.NormalizeUUID(input.TargetIdentityID)
	clientMessageID, clientMessageIDValid := common.NormalizeUUID(input.ClientMessageID)
	body := strings.TrimSpace(input.Body)
	fields := map[string]ValidationCode{}
	if !valid {
		fields["targetIdentityId"] = ValidationTargetIdentityIDInvalid
	}
	if !clientMessageIDValid {
		fields["clientMessageId"] = ValidationClientMessageIDInvalid
	}
	if body == "" {
		fields["body"] = ValidationBodyRequired
	} else if utf8.RuneCountInString(body) > 4000 {
		fields["body"] = ValidationBodyTooLong
	}
	if len(fields) > 0 {
		return FirstDirectTextMessageResult{}, &ValidationError{Fields: fields}
	}
	if targetIdentityID == identity.OrganizationIdentity.ID {
		return FirstDirectTextMessageResult{}, ErrDirectTargetNotFound
	}
	target, err := loadDirectTarget(ctx, a.db, identity.Organization.ID, targetIdentityID)
	if err != nil {
		return FirstDirectTextMessageResult{}, err
	}
	// 预生成单聊创建事务使用的 UUIDv7。
	values := make([]string, 5)
	for index := range values {
		values[index] = uuid.NewV7().String()
	}
	ids := directConversationIDs{
		conversation: values[0], currentSubject: values[1], targetSubject: values[2],
		currentParticipant: values[3], targetParticipant: values[4],
	}
	// 双方始终按规范化身份顺序创建聊天主体。
	firstIdentityID, secondIdentityID := normalizeDirectIdentityPair(identity.OrganizationIdentity.ID, target.IdentityID)
	firstSubjectID, secondSubjectID := ids.currentSubject, ids.targetSubject
	if firstIdentityID != identity.OrganizationIdentity.ID {
		firstSubjectID, secondSubjectID = secondSubjectID, firstSubjectID
	}

	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		var result FirstDirectTextMessageResult
		err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
				return err
			}
			firstSubject, err := ensureOrganizationIdentityChatSubject(ctx, tx, identity.Organization.ID, firstIdentityID, firstSubjectID)
			if err != nil {
				return err
			}
			secondSubject, err := ensureOrganizationIdentityChatSubject(ctx, tx, identity.Organization.ID, secondIdentityID, secondSubjectID)
			if err != nil {
				return err
			}
			currentSubject, targetSubject := firstSubject, secondSubject
			if firstIdentityID != identity.OrganizationIdentity.ID {
				currentSubject, targetSubject = secondSubject, firstSubject
			}
			conversation, err := findDirectConversation(ctx, tx, identity.Organization.ID, identity.OrganizationIdentity.ID, target.IdentityID)
			if err != nil {
				return err
			}
			if conversation == nil {
				conversation, err = createDirectConversation(ctx, tx, identity.Organization.ID, identity.OrganizationIdentity.ID, target.IdentityID, currentSubject.ID, targetSubject.ID, ids)
				if err != nil {
					return err
				}
			} else if conversation.Status == string(domain.ConversationStatusArchived) {
				if _, err := tx.NewUpdate().Model(conversation).
					Set("status = ?", domain.ConversationStatusActive).
					Set("updated_at = now()").
					WherePK().
					Where("organization_id = ?", identity.Organization.ID).
					Exec(ctx); err != nil {
					return fmt.Errorf("reactivate direct conversation: %w", err)
				}
				conversation.Status = string(domain.ConversationStatusActive)
			} else if conversation.Status != string(domain.ConversationStatusActive) {
				return ErrDataInvariant
			}
			message, err := sendDirectTextMessage(ctx, tx, identity, DirectTextMessageInput{
				ConversationID: conversation.ID, ClientMessageID: clientMessageID, Body: body,
			}, a.agentScheduler)
			if err != nil {
				return err
			}
			summary, err := loadDirectConversationSummary(ctx, tx, identity.Organization.ID, conversation.ID, target)
			if err != nil {
				return err
			}
			result = FirstDirectTextMessageResult{Conversation: summary, Message: message}
			return nil
		})
		if err == nil {
			return result, nil
		}
		constraint, retryable := retryableUniqueViolation(err, map[string]struct{}{
			"chat_subjects_organization_kind_source_unique":          {},
			"direct_conversations_organization_identity_pair_unique": {},
			"messages_organization_idempotency_unique":               {},
		})
		if !retryable {
			return FirstDirectTextMessageResult{}, err
		}
		if attempt < maxWriteAttempts-1 {
			slog.Info("内部单聊首条消息写入重试", "target_identity_id", targetIdentityID, "attempt", attempt+2, "constraint", constraint)
		}
	}
	return FirstDirectTextMessageResult{}, fmt.Errorf("send first direct text message retries exhausted: %w", err)
}

// Execute 在可重试事务中写入内部单聊文本消息。
func (a *SendDirectTextMessageAction) Execute(ctx context.Context, identity *servermodels.Identity, input DirectTextMessageInput) (ConversationMessage, error) {
	normalized, fields := normalizeDirectTextMessageInput(input)
	if len(fields) > 0 {
		return ConversationMessage{}, &ValidationError{Fields: fields}
	}
	var err error

	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		var result ConversationMessage
		err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
				return err
			}
			var sendErr error
			result, sendErr = sendDirectTextMessage(ctx, tx, identity, normalized, a.agentScheduler)
			return sendErr
		})
		if err == nil {
			return result, nil
		}
		constraint, retryable := retryableUniqueViolation(err, directMessageRetryableConstraintNames)
		if !retryable {
			return ConversationMessage{}, err
		}
		if attempt < maxWriteAttempts-1 {
			slog.Info("内部单聊消息写入重试", "conversation_id", normalized.ConversationID, "attempt", attempt+2, "constraint", constraint)
		}
	}
	return ConversationMessage{}, fmt.Errorf("send direct message retries exhausted: %w", err)
}

// sendDirectTextMessage 在当前事务中幂等写入单聊文本消息。
func sendDirectTextMessage(ctx context.Context, db bun.IDB, identity *servermodels.Identity, input DirectTextMessageInput, agentScheduler DirectAgentMessageScheduler) (ConversationMessage, error) {
	sendContext, err := loadDirectSendContext(ctx, db, identity, input.ConversationID)
	if err != nil {
		return ConversationMessage{}, err
	}
	idempotencyKey := "mmsg:" + identity.OrganizationIdentity.ID + ":" + input.ClientMessageID
	if saved, found, err := loadIdempotentMemberMessage(ctx, db, identity, input.ConversationID, input.Body, idempotencyKey, false); err != nil || found {
		return saved, err
	}
	message := &servermodels.Message{
		ID: uuid.NewV7().String(), OrganizationID: identity.Organization.ID,
		ConversationID: input.ConversationID, SenderParticipantID: &sendContext.ParticipantID,
		Type: string(domain.MessageTypeText), Body: input.Body,
		IdempotencyKey: &idempotencyKey, OriginatedAt: time.Now().UTC(),
	}
	if _, err := db.NewInsert().Model(message).
		Column("id", "organization_id", "conversation_id", "sender_participant_id", "type", "body", "idempotency_key", "originated_at").
		Returning("*").
		Exec(ctx); err != nil {
		return ConversationMessage{}, fmt.Errorf("create direct text message: %w", err)
	}
	if sendContext.PeerType == domain.OrganizationIdentityTypeAgent {
		if agentScheduler == nil || sendContext.PeerIdentityID == "" || sendContext.PeerRevisionID == nil {
			return ConversationMessage{}, ErrDataInvariant
		}
		if err := agentScheduler.Schedule(ctx, db, identity.Organization.ID, input.ConversationID, sendContext.PeerIdentityID, *sendContext.PeerRevisionID, message.ID); err != nil {
			return ConversationMessage{}, fmt.Errorf("schedule agent direct message: %w", err)
		}
	}
	conversation := &servermodels.Conversation{ID: input.ConversationID, OrganizationID: identity.Organization.ID}
	if err := updateConversationSummary(ctx, db, conversation, message); err != nil {
		return ConversationMessage{}, err
	}
	if err := advanceConversationUserReadState(ctx, db, &servermodels.ConversationUserState{
		OrganizationID: identity.Organization.ID, ConversationID: input.ConversationID,
		UserID: identity.User.ID, LastReadMessageID: &message.ID,
	}, message); err != nil {
		return ConversationMessage{}, err
	}
	return memberConversationMessage(message, sendContext.SubjectID, identity.OrganizationIdentity.ID, identity.OrganizationIdentity.DisplayName), nil
}

// loadDirectTarget 读取同企业可发起单聊的活跃成员身份。
func loadDirectTarget(ctx context.Context, db bun.IDB, organizationID, identityID string) (directTargetRow, error) {
	row := directTargetRow{}
	err := db.NewSelect().
		TableExpr("organization_identities AS oi").
		ColumnExpr("oi.id AS identity_id").
		ColumnExpr("oi.type AS identity_type").
		ColumnExpr("oi.display_name AS display_name").
		Join("LEFT JOIN users AS u ON u.organization_id = oi.organization_id AND u.identity_id = oi.id").
		Join("LEFT JOIN agents AS a ON a.organization_id = oi.organization_id AND a.identity_id = oi.id").
		Where("oi.organization_id = ?", organizationID).
		Where("oi.id = ?", identityID).
		Where("((oi.type = ? AND u.status = ?) OR (oi.type = ? AND a.status = ?))", domain.OrganizationIdentityTypeUser, domain.UserStatusActive, domain.OrganizationIdentityTypeAgent, domain.UserStatusActive).
		Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return directTargetRow{}, ErrDirectTargetNotFound
	}
	if err != nil {
		return directTargetRow{}, fmt.Errorf("load direct target: %w", err)
	}
	return row, nil
}

// normalizeDirectIdentityPair 按稳定顺序排列单聊双方身份。
func normalizeDirectIdentityPair(firstIdentityID, secondIdentityID string) (string, string) {
	identityIDs := []string{firstIdentityID, secondIdentityID}
	sort.Strings(identityIDs)
	return identityIDs[0], identityIDs[1]
}

// ensureOrganizationIdentityChatSubject 取得或创建企业身份聊天主体。
func ensureOrganizationIdentityChatSubject(ctx context.Context, db bun.IDB, organizationID, identityID, subjectID string) (*servermodels.ChatSubject, error) {
	subject := &servermodels.ChatSubject{}
	err := db.NewSelect().Model(subject).
		Where("cs.organization_id = ?", organizationID).
		Where("cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Where("cs.source_id = ?", identityID).
		Scan(ctx)
	if err == nil {
		return subject, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("find organization identity chat subject: %w", err)
	}
	subject = &servermodels.ChatSubject{
		ID: subjectID, OrganizationID: organizationID,
		Kind: string(domain.ChatSubjectKindOrganizationIdentity), SourceID: identityID,
	}
	if _, err := db.NewInsert().Model(subject).
		Column("id", "organization_id", "kind", "source_id").
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("create organization identity chat subject: %w", err)
	}
	return subject, nil
}

// findDirectConversation 查找规范身份对唯一的长期单聊。
func findDirectConversation(ctx context.Context, db bun.IDB, organizationID, firstIdentityID, secondIdentityID string) (*servermodels.Conversation, error) {
	firstIdentityID, secondIdentityID = normalizeDirectIdentityPair(firstIdentityID, secondIdentityID)
	conversation := &servermodels.Conversation{}
	err := db.NewSelect().Model(conversation).
		Join("JOIN direct_conversations AS dc ON dc.organization_id = cv.organization_id AND dc.conversation_id = cv.id").
		Where("cv.organization_id = ?", organizationID).
		Where("cv.type = ?", domain.ConversationTypeDirect).
		Where("dc.first_identity_id = ?", firstIdentityID).
		Where("dc.second_identity_id = ?", secondIdentityID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find direct conversation: %w", err)
	}
	return conversation, nil
}

// createDirectConversation 创建内部单聊和双方参与者。
func createDirectConversation(ctx context.Context, db bun.IDB, organizationID, currentIdentityID, targetIdentityID, currentSubjectID, targetSubjectID string, ids directConversationIDs) (*servermodels.Conversation, error) {
	conversation := &servermodels.Conversation{
		ID: ids.conversation, OrganizationID: organizationID,
		Type: string(domain.ConversationTypeDirect), Status: string(domain.ConversationStatusActive),
		CreatedBySubjectID: &currentSubjectID,
	}
	if _, err := db.NewInsert().Model(conversation).
		Column("id", "organization_id", "type", "status", "created_by_subject_id").
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("create direct conversation: %w", err)
	}
	firstIdentityID, secondIdentityID := normalizeDirectIdentityPair(currentIdentityID, targetIdentityID)
	relation := &servermodels.DirectConversation{
		ConversationID: conversation.ID, OrganizationID: organizationID,
		FirstIdentityID: firstIdentityID, SecondIdentityID: secondIdentityID,
	}
	if _, err := db.NewInsert().Model(relation).
		Column("conversation_id", "organization_id", "first_identity_id", "second_identity_id").
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("create direct conversation relation: %w", err)
	}
	participants := []*servermodels.ConversationParticipant{
		{ID: ids.currentParticipant, OrganizationID: organizationID, ConversationID: conversation.ID, SubjectID: currentSubjectID, Role: string(domain.ConversationParticipantRoleMember)},
		{ID: ids.targetParticipant, OrganizationID: organizationID, ConversationID: conversation.ID, SubjectID: targetSubjectID, Role: string(domain.ConversationParticipantRoleMember)},
	}
	if _, err := db.NewInsert().Model(&participants).
		Column("id", "organization_id", "conversation_id", "subject_id", "role").
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("create direct conversation participants: %w", err)
	}
	return conversation, nil
}

// loadDirectConversationSummary 读取单聊当前摘要。
func loadDirectConversationSummary(ctx context.Context, db bun.IDB, organizationID, conversationID string, target directTargetRow) (DirectConversationSummary, error) {
	row := directConversationSummaryRow{}
	err := db.NewSelect().
		TableExpr("conversations AS cv").
		ColumnExpr("cv.id AS id").
		ColumnExpr("msg.body AS preview").
		ColumnExpr("cv.last_message_at AS last_message_at").
		Join("LEFT JOIN messages AS msg ON msg.organization_id = cv.organization_id AND msg.conversation_id = cv.id AND msg.id = cv.last_message_id AND msg.deleted_at IS NULL").
		Where("cv.organization_id = ?", organizationID).
		Where("cv.id = ?", conversationID).
		Where("cv.type = ?", domain.ConversationTypeDirect).
		Scan(ctx, &row)
	if err != nil {
		return DirectConversationSummary{}, fmt.Errorf("load direct conversation summary: %w", err)
	}
	return DirectConversationSummary{
		ID: row.ID, PeerIdentityID: target.IdentityID, PeerType: target.IdentityType, PeerName: target.DisplayName,
		Preview: row.Preview, LastMessageAt: row.LastMessageAt,
	}, nil
}

// loadDirectSendContext 校验当前成员是单聊现有有效参与者。
func loadDirectSendContext(ctx context.Context, db bun.IDB, identity *servermodels.Identity, conversationID string) (directSendContextRow, error) {
	row := directSendContextRow{}
	err := db.NewSelect().
		TableExpr("conversations AS cv").
		ColumnExpr("cv.id AS conversation_id").
		ColumnExpr("mine.id AS participant_id").
		ColumnExpr("mine_cs.id AS subject_id").
		ColumnExpr("peer_oi.type AS peer_type").
		ColumnExpr("peer_oi.id AS peer_identity_id").
		ColumnExpr("peer_a.active_revision_id AS peer_revision_id").
		Join("JOIN direct_conversations AS dc ON dc.organization_id = cv.organization_id AND dc.conversation_id = cv.id").
		Join("JOIN conversation_participants AS mine ON mine.organization_id = cv.organization_id AND mine.conversation_id = cv.id AND mine.left_at IS NULL").
		Join("JOIN chat_subjects AS mine_cs ON mine_cs.organization_id = mine.organization_id AND mine_cs.id = mine.subject_id AND mine_cs.kind = ? AND mine_cs.source_id = ?", domain.ChatSubjectKindOrganizationIdentity, identity.OrganizationIdentity.ID).
		Join("JOIN organization_identities AS peer_oi ON peer_oi.organization_id = dc.organization_id AND peer_oi.id = CASE WHEN dc.first_identity_id = ? THEN dc.second_identity_id ELSE dc.first_identity_id END", identity.OrganizationIdentity.ID).
		Join("LEFT JOIN users AS peer_u ON peer_u.organization_id = peer_oi.organization_id AND peer_u.identity_id = peer_oi.id").
		Join("LEFT JOIN agents AS peer_a ON peer_a.organization_id = peer_oi.organization_id AND peer_a.identity_id = peer_oi.id").
		Where("cv.organization_id = ?", identity.Organization.ID).
		Where("cv.id = ?", conversationID).
		Where("cv.type = ?", domain.ConversationTypeDirect).
		Where("cv.status = ?", domain.ConversationStatusActive).
		Where("? IN (dc.first_identity_id, dc.second_identity_id)", identity.OrganizationIdentity.ID).
		Where("((peer_oi.type = ? AND peer_u.status = ?) OR (peer_oi.type = ? AND peer_a.status = ?))", domain.OrganizationIdentityTypeUser, domain.UserStatusActive, domain.OrganizationIdentityTypeAgent, domain.UserStatusActive).
		Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return directSendContextRow{}, ErrConversationNotFound
	}
	if err != nil {
		return directSendContextRow{}, fmt.Errorf("load direct send context: %w", err)
	}
	return row, nil
}

// normalizeDirectTextMessageInput 规范化并校验内部单聊文本消息输入。
func normalizeDirectTextMessageInput(input DirectTextMessageInput) (DirectTextMessageInput, map[string]ValidationCode) {
	conversationID, clientMessageID, body, fields := normalizeInternalTextMessageInput(input.ConversationID, input.ClientMessageID, input.Body)
	input.ConversationID = conversationID
	input.ClientMessageID = clientMessageID
	input.Body = body
	return input, fields
}

// normalizeInternalTextMessageInput 规范化内部会话文本消息输入。
func normalizeInternalTextMessageInput(conversationID, clientMessageID, body string) (string, string, string, map[string]ValidationCode) {
	fields := map[string]ValidationCode{}
	body = strings.TrimSpace(body)
	var valid bool
	conversationID, valid = common.NormalizeUUID(conversationID)
	if !valid {
		fields["conversationId"] = ValidationConversationIDInvalid
	}
	clientMessageID, valid = common.NormalizeUUID(clientMessageID)
	if !valid {
		fields["clientMessageId"] = ValidationClientMessageIDInvalid
	}
	if body == "" {
		fields["body"] = ValidationBodyRequired
	} else if utf8.RuneCountInString(body) > 4000 {
		fields["body"] = ValidationBodyTooLong
	}
	return conversationID, clientMessageID, body, fields
}
