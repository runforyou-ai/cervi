//go:build server

package conversation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
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

// StartDirectConversationAction 发起或打开企业成员内部单聊。
type StartDirectConversationAction struct {
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

// NewStartDirectConversationAction 创建内部单聊发起操作。
func NewStartDirectConversationAction(db *bun.DB) *StartDirectConversationAction {
	return &StartDirectConversationAction{db: db}
}

// NewSendDirectTextMessageAction 创建内部单聊发送操作。
func NewSendDirectTextMessageAction(db *bun.DB, agentScheduler DirectAgentMessageScheduler) *SendDirectTextMessageAction {
	return &SendDirectTextMessageAction{db: db, agentScheduler: agentScheduler}
}

// Execute 发起或打开当前成员与目标成员的长期单聊。
func (a *StartDirectConversationAction) Execute(ctx context.Context, identity *servermodels.Identity, input DirectConversationInput) (DirectConversationSummary, error) {
	targetIdentityID, valid := common.NormalizeUUID(input.TargetIdentityID)
	if !valid {
		return DirectConversationSummary{}, &ValidationError{Fields: map[string]ValidationCode{
			"targetIdentityId": ValidationTargetIdentityIDInvalid,
		}}
	}
	if targetIdentityID == identity.OrganizationIdentity.ID {
		return DirectConversationSummary{}, ErrDirectTargetNotFound
	}
	target, err := loadDirectTarget(ctx, a.db, identity.Organization.ID, targetIdentityID)
	if err != nil {
		return DirectConversationSummary{}, err
	}
	ids := generateDirectConversationIDs()

	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		var result DirectConversationSummary
		err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
				return err
			}
			if err := lockDirectIdentityPair(ctx, tx, identity.Organization.ID, identity.OrganizationIdentity.ID, target.IdentityID); err != nil {
				return err
			}
			currentSubject, err := ensureOrganizationIdentityChatSubject(ctx, tx, identity.Organization.ID, identity.OrganizationIdentity.ID, ids.currentSubject)
			if err != nil {
				return err
			}
			targetSubject, err := ensureOrganizationIdentityChatSubject(ctx, tx, identity.Organization.ID, target.IdentityID, ids.targetSubject)
			if err != nil {
				return err
			}
			conversation, err := findDirectConversation(ctx, tx, identity.Organization.ID, currentSubject.ID, targetSubject.ID)
			if err != nil {
				return err
			}
			if conversation == nil {
				conversation, err = createDirectConversation(ctx, tx, identity.Organization.ID, currentSubject.ID, targetSubject.ID, ids)
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
			result, err = loadDirectConversationSummary(ctx, tx, identity.Organization.ID, conversation.ID, target)
			return err
		})
		if err == nil {
			return result, nil
		}
		constraint, retryable := retryableUniqueViolation(err, map[string]struct{}{
			"chat_subjects_organization_kind_source_unique": {},
		})
		if !retryable {
			return DirectConversationSummary{}, err
		}
		if attempt < maxWriteAttempts-1 {
			slog.Info("内部单聊发起重试", "target_identity_id", targetIdentityID, "attempt", attempt+2, "constraint", constraint)
		}
	}
	return DirectConversationSummary{}, fmt.Errorf("start direct conversation retries exhausted: %w", err)
}

// Execute 在可重试事务中写入内部单聊文本消息。
func (a *SendDirectTextMessageAction) Execute(ctx context.Context, identity *servermodels.Identity, input DirectTextMessageInput) (ConversationMessage, error) {
	normalized, fields := normalizeDirectTextMessageInput(input)
	if len(fields) > 0 {
		return ConversationMessage{}, &ValidationError{Fields: fields}
	}
	messageID := uuid.NewV7()
	originatedAt := time.Now().UTC()
	idempotencyKey := "mmsg:" + identity.OrganizationIdentity.ID + ":" + normalized.ClientMessageID
	var err error

	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		var result ConversationMessage
		err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
				return err
			}
			sendContext, err := loadDirectSendContext(ctx, tx, identity, normalized.ConversationID)
			if err != nil {
				return err
			}
			if saved, found, err := loadIdempotentMemberMessage(ctx, tx, identity, normalized.ConversationID, normalized.Body, idempotencyKey, false); err != nil || found {
				result = saved
				return err
			}
			message := &servermodels.Message{
				ID: messageID.String(), OrganizationID: identity.Organization.ID,
				ConversationID: normalized.ConversationID, SenderParticipantID: &sendContext.ParticipantID,
				Type: string(domain.MessageTypeText), Body: normalized.Body,
				IdempotencyKey: &idempotencyKey, OriginatedAt: originatedAt,
			}
			if _, err := tx.NewInsert().Model(message).
				Column("id", "organization_id", "conversation_id", "sender_participant_id", "type", "body", "idempotency_key", "originated_at").
				Returning("*").
				Exec(ctx); err != nil {
				return fmt.Errorf("create direct text message: %w", err)
			}
			if sendContext.PeerType == domain.OrganizationIdentityTypeAgent {
				if a.agentScheduler == nil || sendContext.PeerIdentityID == "" || sendContext.PeerRevisionID == nil {
					return ErrDataInvariant
				}
				if err := a.agentScheduler.Schedule(
					ctx, tx, identity.Organization.ID, normalized.ConversationID,
					sendContext.PeerIdentityID, *sendContext.PeerRevisionID, message.ID,
				); err != nil {
					return fmt.Errorf("schedule agent direct message: %w", err)
				}
			}
			conversation := &servermodels.Conversation{ID: normalized.ConversationID, OrganizationID: identity.Organization.ID}
			if err := updateConversationSummary(ctx, tx, conversation, message); err != nil {
				return err
			}
			result = memberConversationMessage(message, sendContext.SubjectID, identity.OrganizationIdentity.ID, identity.OrganizationIdentity.DisplayName)
			return nil
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

// lockDirectIdentityPair 串行化同企业同一规范化身份对的单聊创建。
func lockDirectIdentityPair(ctx context.Context, db bun.IDB, organizationID, firstIdentityID, secondIdentityID string) error {
	keyText := directIdentityPairLockText(organizationID, firstIdentityID, secondIdentityID)
	hash := sha256.Sum256([]byte(keyText))
	lockKey := int64(binary.BigEndian.Uint64(hash[:8]))
	if _, err := db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?)", lockKey); err != nil {
		return fmt.Errorf("lock direct identity pair: %w", err)
	}
	return nil
}

// directIdentityPairLockText 生成与发起方向无关的稳定锁文本。
func directIdentityPairLockText(organizationID, firstIdentityID, secondIdentityID string) string {
	identityIDs := []string{firstIdentityID, secondIdentityID}
	sort.Strings(identityIDs)
	return "cervi:direct:" + organizationID + ":" + identityIDs[0] + ":" + identityIDs[1]
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

// findDirectConversation 查找规范主体对唯一的长期单聊。
func findDirectConversation(ctx context.Context, db bun.IDB, organizationID, firstSubjectID, secondSubjectID string) (*servermodels.Conversation, error) {
	var conversations []servermodels.Conversation
	err := db.NewSelect().Model(&conversations).
		Join("JOIN conversation_participants AS first_cp ON first_cp.organization_id = cv.organization_id AND first_cp.conversation_id = cv.id AND first_cp.subject_id = ?", firstSubjectID).
		Join("JOIN conversation_participants AS second_cp ON second_cp.organization_id = cv.organization_id AND second_cp.conversation_id = cv.id AND second_cp.subject_id = ?", secondSubjectID).
		Where("cv.organization_id = ?", organizationID).
		Where("cv.type = ?", domain.ConversationTypeDirect).
		Where("(SELECT count(*) FROM conversation_participants AS all_cp WHERE all_cp.organization_id = cv.organization_id AND all_cp.conversation_id = cv.id) = 2").
		OrderExpr("cv.id ASC").
		Limit(2).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("find direct conversation: %w", err)
	}
	if len(conversations) > 1 {
		return nil, ErrDataInvariant
	}
	if len(conversations) == 0 {
		return nil, nil
	}
	return &conversations[0], nil
}

// createDirectConversation 创建内部单聊和双方参与者。
func createDirectConversation(ctx context.Context, db bun.IDB, organizationID, currentSubjectID, targetSubjectID string, ids directConversationIDs) (*servermodels.Conversation, error) {
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
		Join("JOIN conversation_participants AS mine ON mine.organization_id = cv.organization_id AND mine.conversation_id = cv.id AND mine.left_at IS NULL").
		Join("JOIN chat_subjects AS mine_cs ON mine_cs.organization_id = mine.organization_id AND mine_cs.id = mine.subject_id AND mine_cs.kind = ? AND mine_cs.source_id = ?", domain.ChatSubjectKindOrganizationIdentity, identity.OrganizationIdentity.ID).
		Join("JOIN conversation_participants AS peer ON peer.organization_id = cv.organization_id AND peer.conversation_id = cv.id AND peer.subject_id <> mine.subject_id AND peer.left_at IS NULL").
		Join("JOIN chat_subjects AS peer_cs ON peer_cs.organization_id = peer.organization_id AND peer_cs.id = peer.subject_id AND peer_cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("JOIN organization_identities AS peer_oi ON peer_oi.organization_id = peer_cs.organization_id AND peer_oi.id = peer_cs.source_id").
		Join("LEFT JOIN users AS peer_u ON peer_u.organization_id = peer_oi.organization_id AND peer_u.identity_id = peer_oi.id").
		Join("LEFT JOIN agents AS peer_a ON peer_a.organization_id = peer_oi.organization_id AND peer_a.identity_id = peer_oi.id").
		Where("cv.organization_id = ?", identity.Organization.ID).
		Where("cv.id = ?", conversationID).
		Where("cv.type = ?", domain.ConversationTypeDirect).
		Where("cv.status = ?", domain.ConversationStatusActive).
		Where("((peer_oi.type = ? AND peer_u.status = ?) OR (peer_oi.type = ? AND peer_a.status = ?))", domain.OrganizationIdentityTypeUser, domain.UserStatusActive, domain.OrganizationIdentityTypeAgent, domain.UserStatusActive).
		Where("(SELECT count(*) FROM conversation_participants AS all_cp WHERE all_cp.organization_id = cv.organization_id AND all_cp.conversation_id = cv.id) = 2").
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

// generateDirectConversationIDs 预生成单聊创建事务使用的 UUIDv7。
func generateDirectConversationIDs() directConversationIDs {
	values := make([]string, 5)
	for index := range values {
		values[index] = uuid.NewV7().String()
	}
	return directConversationIDs{
		conversation: values[0], currentSubject: values[1], targetSubject: values[2],
		currentParticipant: values[3], targetParticipant: values[4],
	}
}
