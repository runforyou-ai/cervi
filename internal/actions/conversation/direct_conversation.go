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

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
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
	db *bun.DB
}

// FindDirectConversationQuery 按目标身份查找当前成员的活跃长期单聊。
type FindDirectConversationQuery struct {
	db *bun.DB
}

// SendDirectTextMessageAction 持久化企业成员内部单聊文本消息。
type SendDirectTextMessageAction struct {
	db *bun.DB
}

type directTargetRow struct {
	IdentityID   string                          `bun:"identity_id"`
	IdentityType domain.OrganizationIdentityType `bun:"identity_type"`
	DisplayName  string                          `bun:"display_name"`
	AvatarFileID *string                         `bun:"avatar_file_id"`
}

type directConversationSummaryRow struct {
	ID                        string                           `bun:"id"`
	Preview                   *string                          `bun:"preview"`
	PreviewSenderIdentityType *domain.OrganizationIdentityType `bun:"preview_sender_identity_type"`
	LastMessageAt             *time.Time                       `bun:"last_message_at"`
}

// NewSendFirstDirectTextMessageAction 创建首条单聊消息发送操作。
func NewSendFirstDirectTextMessageAction(db *bun.DB) *SendFirstDirectTextMessageAction {
	return &SendFirstDirectTextMessageAction{db: db}
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
func NewSendDirectTextMessageAction(db *bun.DB) *SendDirectTextMessageAction {
	return &SendDirectTextMessageAction{db: db}
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
	var err error
	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		var result FirstDirectTextMessageResult
		err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
				return err
			}
			target, err := loadDirectTarget(ctx, tx, identity.Organization.ID, targetIdentityID)
			if err != nil {
				return err
			}
			conversation, err := findDirectConversation(ctx, tx, identity.Organization.ID, identity.OrganizationIdentity.ID, targetIdentityID)
			if err != nil {
				return err
			}
			if conversation == nil {
				conversation, err = createDirectConversation(ctx, tx, identity.Organization.ID, identity.OrganizationIdentity.ID, targetIdentityID)
				if err != nil {
					return err
				}
			}
			message, err := sendDirectTextMessage(ctx, tx, identity, InternalTextMessageInput{
				ConversationID: conversation.ID, ClientMessageID: clientMessageID, Body: body,
			}, true)
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
func (a *SendDirectTextMessageAction) Execute(ctx context.Context, identity *servermodels.Identity, input InternalTextMessageInput) (ConversationMessage, error) {
	normalized, fields := normalizeInternalMessageInput(input)
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
			result, sendErr = sendDirectTextMessage(ctx, tx, identity, normalized, false)
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

// sendDirectTextMessage 锁定真人单聊并按显式首发意图恢复归档会话。
func sendDirectTextMessage(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, input InternalTextMessageInput, restoreArchived bool) (ConversationMessage, error) {
	member, err := chatstate.LockMember(ctx, tx, identity, input.ConversationID)
	if err != nil {
		return ConversationMessage{}, err
	}
	conversation := member.Conversation
	if conversation.Type != string(domain.ConversationTypeDirect) {
		return ConversationMessage{}, ErrConversationNotFound
	}
	if restoreArchived && conversation.Status == string(domain.ConversationStatusArchived) {
		if _, err := tx.NewUpdate().Model(conversation).
			Set("status = ?", domain.ConversationStatusActive).
			Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
			return ConversationMessage{}, fmt.Errorf("reactivate direct conversation: %w", err)
		}
	}
	// 等待会话锁后重新读取目标资格，幂等重放也需通过当前发送授权。
	sendContext, err := loadDirectSendContext(ctx, tx, identity, input.ConversationID)
	if err != nil {
		return ConversationMessage{}, err
	}
	return saveInternalTextMessage(ctx, tx, identity, input, sendContext, nil)
}

// loadDirectTarget 读取同企业可发起单聊的活跃成员身份。
func loadDirectTarget(ctx context.Context, db bun.IDB, organizationID, identityID string) (directTargetRow, error) {
	row := directTargetRow{}
	err := db.NewSelect().
		TableExpr("organization_identities AS oi").
		ColumnExpr("oi.id AS identity_id").
		ColumnExpr("oi.type AS identity_type").
		ColumnExpr("oi.display_name AS display_name").
		ColumnExpr("oi.avatar_file_id::text AS avatar_file_id").
		Join("LEFT JOIN users AS u ON u.organization_id = oi.organization_id AND u.identity_id = oi.id").
		Where("oi.organization_id = ?", organizationID).
		Where("oi.id = ?", identityID).
		Where("oi.type = ? AND u.status = ?", domain.OrganizationIdentityTypeUser, domain.UserStatusActive).
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
func createDirectConversation(ctx context.Context, db bun.IDB, organizationID, currentIdentityID, targetIdentityID string) (*servermodels.Conversation, error) {
	subjects, err := ensureOrganizationIdentityChatSubjects(ctx, db, organizationID, []string{currentIdentityID, targetIdentityID})
	if err != nil {
		return nil, err
	}
	currentSubjectID, targetSubjectID := subjects[currentIdentityID].ID, subjects[targetIdentityID].ID
	conversation := &servermodels.Conversation{
		ID: uuid.NewV7().String(), OrganizationID: organizationID,
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
		{ID: uuid.NewV7().String(), OrganizationID: organizationID, ConversationID: conversation.ID, SubjectID: currentSubjectID, Role: string(domain.ConversationParticipantRoleMember)},
		{ID: uuid.NewV7().String(), OrganizationID: organizationID, ConversationID: conversation.ID, SubjectID: targetSubjectID, Role: string(domain.ConversationParticipantRoleMember)},
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
		ColumnExpr("CASE WHEN msg.type = ? THEN (SELECT ma.name FROM message_attachments ma WHERE ma.message_id = msg.id AND ma.organization_id = msg.organization_id) ELSE msg.body END AS preview", domain.MessageTypeAttachment).
		ColumnExpr("preview_oi.type AS preview_sender_identity_type").
		ColumnExpr("cv.last_message_at AS last_message_at").
		Join("LEFT JOIN messages AS msg ON msg.organization_id = cv.organization_id AND msg.conversation_id = cv.id AND msg.id = cv.last_message_id AND msg.deleted_at IS NULL").
		Join("LEFT JOIN conversation_participants AS preview_cp ON preview_cp.id = msg.sender_participant_id AND preview_cp.organization_id = msg.organization_id AND preview_cp.conversation_id = msg.conversation_id").
		Join("LEFT JOIN chat_subjects AS preview_cs ON preview_cs.id = preview_cp.subject_id AND preview_cs.organization_id = preview_cp.organization_id").
		Join("LEFT JOIN organization_identities AS preview_oi ON preview_oi.id = preview_cs.source_id AND preview_oi.organization_id = preview_cs.organization_id AND preview_cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Where("cv.organization_id = ?", organizationID).
		Where("cv.id = ?", conversationID).
		Where("cv.type = ?", domain.ConversationTypeDirect).
		Scan(ctx, &row)
	if err != nil {
		return DirectConversationSummary{}, fmt.Errorf("load direct conversation summary: %w", err)
	}
	return DirectConversationSummary{
		ID: row.ID, PeerIdentityID: target.IdentityID, PeerType: target.IdentityType, PeerName: target.DisplayName, PeerAvatarFileID: target.AvatarFileID,
		Preview: row.Preview, PreviewSenderIdentityType: row.PreviewSenderIdentityType, LastMessageAt: row.LastMessageAt,
	}, nil
}

// loadDirectSendContext 校验当前成员是单聊现有有效参与者。
func loadDirectSendContext(ctx context.Context, db bun.IDB, identity *servermodels.Identity, conversationID string) (internalMessageContext, error) {
	row := internalMessageContext{}
	err := db.NewSelect().
		TableExpr("conversations AS cv").
		ColumnExpr("cv.id AS conversation_id").
		ColumnExpr("mine.id AS participant_id").
		ColumnExpr("mine_cs.id AS subject_id").
		Join("JOIN direct_conversations AS dc ON dc.organization_id = cv.organization_id AND dc.conversation_id = cv.id").
		Join("JOIN conversation_participants AS mine ON mine.organization_id = cv.organization_id AND mine.conversation_id = cv.id AND mine.left_at IS NULL").
		Join("JOIN chat_subjects AS mine_cs ON mine_cs.organization_id = mine.organization_id AND mine_cs.id = mine.subject_id AND mine_cs.kind = ? AND mine_cs.source_id = ?", domain.ChatSubjectKindOrganizationIdentity, identity.OrganizationIdentity.ID).
		Join("JOIN organization_identities AS peer_oi ON peer_oi.organization_id = dc.organization_id AND peer_oi.id = CASE WHEN dc.first_identity_id = ? THEN dc.second_identity_id ELSE dc.first_identity_id END", identity.OrganizationIdentity.ID).
		Join("LEFT JOIN users AS peer_u ON peer_u.organization_id = peer_oi.organization_id AND peer_u.identity_id = peer_oi.id").
		Where("cv.organization_id = ?", identity.Organization.ID).
		Where("cv.id = ?", conversationID).
		Where("cv.type = ?", domain.ConversationTypeDirect).
		Where("cv.status = ?", domain.ConversationStatusActive).
		Where("? IN (dc.first_identity_id, dc.second_identity_id)", identity.OrganizationIdentity.ID).
		Where("peer_oi.type = ? AND peer_u.status = ?", domain.OrganizationIdentityTypeUser, domain.UserStatusActive).
		Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return internalMessageContext{}, ErrConversationNotFound
	}
	if err != nil {
		return internalMessageContext{}, fmt.Errorf("load direct send context: %w", err)
	}
	return row, nil
}
