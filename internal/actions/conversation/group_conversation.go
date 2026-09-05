//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
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

const (
	maxGroupTitleLength       = 100
	maxGroupDescriptionLength = 500
	maxGroupParticipantCount  = 100
)

// CreateGroupConversationAction 创建企业内部群聊和初始成员关系。
type CreateGroupConversationAction struct {
	db *bun.DB
}

// GetGroupConversationQuery 读取企业内部群聊资料和当前成员。
type GetGroupConversationQuery struct {
	db *bun.DB
}

// SendGroupTextMessageAction 持久化企业内部群聊文本消息。
type SendGroupTextMessageAction struct {
	db *bun.DB
}

type groupMemberRow struct {
	IdentityID   string  `bun:"identity_id"`
	DisplayName  string  `bun:"display_name"`
	AvatarFileID *string `bun:"avatar_file_id"`
}

type groupParticipantRow struct {
	ChatSubjectID string  `bun:"chat_subject_id"`
	IdentityID    string  `bun:"identity_id"`
	DisplayName   string  `bun:"display_name"`
	AvatarFileID  *string `bun:"avatar_file_id"`
	Role          string  `bun:"role"`
}

type groupSendContextRow struct {
	ConversationID string `bun:"conversation_id"`
	ParticipantID  string `bun:"participant_id"`
	SubjectID      string `bun:"subject_id"`
}

// NewCreateGroupConversationAction 创建群聊创建操作。
func NewCreateGroupConversationAction(db *bun.DB) *CreateGroupConversationAction {
	return &CreateGroupConversationAction{db: db}
}

// NewGetGroupConversationQuery 创建群聊资料查询。
func NewGetGroupConversationQuery(db *bun.DB) *GetGroupConversationQuery {
	return &GetGroupConversationQuery{db: db}
}

// NewSendGroupTextMessageAction 创建群聊文本发送操作。
func NewSendGroupTextMessageAction(db *bun.DB) *SendGroupTextMessageAction {
	return &SendGroupTextMessageAction{db: db}
}

// Execute 创建只包含有效真人成员的企业内部群聊。
func (a *CreateGroupConversationAction) Execute(ctx context.Context, identity *servermodels.Identity, input GroupConversationInput) (GroupConversationSummary, error) {
	normalized, fields := normalizeGroupConversationInput(identity.OrganizationIdentity.ID, input)
	if len(fields) > 0 {
		return GroupConversationSummary{}, &ValidationError{Fields: fields}
	}

	conversationID := uuid.NewV7().String()
	subjectIDs := make(map[string]string, len(normalized.MemberIdentityIDs)+1)
	participantIDs := make(map[string]string, len(normalized.MemberIdentityIDs)+1)
	for _, identityID := range append([]string{identity.OrganizationIdentity.ID}, normalized.MemberIdentityIDs...) {
		subjectIDs[identityID] = uuid.NewV7().String()
		participantIDs[identityID] = uuid.NewV7().String()
	}

	var err error
	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
				return err
			}
			members, err := loadActiveGroupMembers(ctx, tx, identity.Organization.ID, normalized.MemberIdentityIDs)
			if err != nil {
				return err
			}
			var imageFileID *string
			if normalized.ImageFileID != "" {
				imageFileID, err = activateGroupImage(ctx, tx, identity.Organization.ID, normalized.ImageFileID, nil)
				if err != nil {
					return err
				}
			}
			creatorSubject, err := ensureOrganizationIdentityChatSubject(ctx, tx, identity.Organization.ID, identity.OrganizationIdentity.ID, subjectIDs[identity.OrganizationIdentity.ID])
			if err != nil {
				return err
			}
			createdBySubjectID := creatorSubject.ID
			conversation := &servermodels.Conversation{
				ID: conversationID, OrganizationID: identity.Organization.ID,
				Type: string(domain.ConversationTypeGroup), Status: string(domain.ConversationStatusActive),
				Title: &normalized.Title, Description: common.OptionalString(normalized.Description),
				ImageFileID: imageFileID, CreatedBySubjectID: &createdBySubjectID,
			}
			if _, err := tx.NewInsert().Model(conversation).
				Column("id", "organization_id", "type", "status", "title", "description", "image_file_id", "created_by_subject_id").
				Exec(ctx); err != nil {
				return fmt.Errorf("create group conversation: %w", err)
			}

			participants := make([]*servermodels.ConversationParticipant, 0, len(members)+1)
			participants = append(participants, &servermodels.ConversationParticipant{
				ID: participantIDs[identity.OrganizationIdentity.ID], OrganizationID: identity.Organization.ID,
				ConversationID: conversation.ID, SubjectID: creatorSubject.ID,
				Role: string(domain.ConversationParticipantRoleOwner),
			})
			for _, member := range members {
				subject, err := ensureOrganizationIdentityChatSubject(ctx, tx, identity.Organization.ID, member.IdentityID, subjectIDs[member.IdentityID])
				if err != nil {
					return err
				}
				participants = append(participants, &servermodels.ConversationParticipant{
					ID: participantIDs[member.IdentityID], OrganizationID: identity.Organization.ID,
					ConversationID: conversation.ID, SubjectID: subject.ID,
					Role: string(domain.ConversationParticipantRoleMember),
				})
			}
			if _, err := tx.NewInsert().Model(&participants).
				Column("id", "organization_id", "conversation_id", "subject_id", "role").
				Exec(ctx); err != nil {
				return fmt.Errorf("create group conversation participants: %w", err)
			}
			return nil
		})
		if err == nil {
			return GroupConversationSummary{
				ID: conversationID, Title: normalized.Title,
				Status: domain.ConversationStatusActive, MemberCount: len(normalized.MemberIdentityIDs) + 1,
				ImageFileID: common.OptionalString(normalized.ImageFileID),
			}, nil
		}
		constraint, retryable := retryableUniqueViolation(err, map[string]struct{}{
			"chat_subjects_organization_kind_source_unique": {},
		})
		if !retryable {
			return GroupConversationSummary{}, err
		}
		if attempt < maxWriteAttempts-1 {
			slog.Info("企业群聊创建重试", "conversation_id", conversationID, "attempt", attempt+2, "constraint", constraint)
		}
	}
	return GroupConversationSummary{}, fmt.Errorf("create group conversation retries exhausted: %w", err)
}

// Execute 委托共用查询返回当前成员可见的群聊。
func (q *GetGroupConversationQuery) Execute(ctx context.Context, identity *servermodels.Identity, conversationID string) (GroupConversation, error) {
	return loadGroupConversation(ctx, q.db, identity, conversationID)
}

// loadGroupConversation 返回当前成员可见的群聊资料和有效参与者。
func loadGroupConversation(ctx context.Context, db bun.IDB, identity *servermodels.Identity, conversationID string) (GroupConversation, error) {
	conversationID, valid := common.NormalizeUUID(conversationID)
	if !valid {
		return GroupConversation{}, &ValidationError{Fields: map[string]ValidationCode{
			"conversationId": ValidationConversationIDInvalid,
		}}
	}
	var summary struct {
		Title       string    `bun:"title"`
		Description string    `bun:"description"`
		ImageFileID *string   `bun:"image_file_id"`
		Status      string    `bun:"status"`
		CreatedAt   time.Time `bun:"created_at"`
	}
	err := db.NewSelect().
		TableExpr("conversations AS cv").
		ColumnExpr("cv.title AS title").
		ColumnExpr("COALESCE(cv.description, '') AS description").
		ColumnExpr("cv.image_file_id::text AS image_file_id").
		ColumnExpr("cv.status AS status").
		ColumnExpr("cv.created_at AS created_at").
		Join("JOIN conversation_participants AS mine ON mine.organization_id = cv.organization_id AND mine.conversation_id = cv.id AND mine.left_at IS NULL").
		Join("JOIN chat_subjects AS mine_cs ON mine_cs.organization_id = mine.organization_id AND mine_cs.id = mine.subject_id AND mine_cs.kind = ? AND mine_cs.source_id = ?", domain.ChatSubjectKindOrganizationIdentity, identity.OrganizationIdentity.ID).
		Where("cv.organization_id = ?", identity.Organization.ID).
		Where("cv.id = ?", conversationID).
		Where("cv.type = ?", domain.ConversationTypeGroup).
		Where("cv.status IN (?, ?)", domain.ConversationStatusActive, domain.ConversationStatusArchived).
		Scan(ctx, &summary)
	if errors.Is(err, sql.ErrNoRows) {
		return GroupConversation{}, ErrConversationNotFound
	}
	if err != nil {
		return GroupConversation{}, fmt.Errorf("load group conversation: %w", err)
	}

	rows := make([]groupParticipantRow, 0)
	if err := db.NewSelect().
		TableExpr("conversation_participants AS cp").
		ColumnExpr("cs.id AS chat_subject_id").
		ColumnExpr("cs.source_id AS identity_id").
		ColumnExpr("oi.display_name AS display_name").
		ColumnExpr("oi.avatar_file_id::text AS avatar_file_id").
		ColumnExpr("cp.role AS role").
		Join("JOIN chat_subjects AS cs ON cs.organization_id = cp.organization_id AND cs.id = cp.subject_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("JOIN organization_identities AS oi ON oi.organization_id = cs.organization_id AND oi.id = cs.source_id").
		Where("cp.organization_id = ?", identity.Organization.ID).
		Where("cp.conversation_id = ?", conversationID).
		Where("cp.left_at IS NULL").
		OrderExpr("CASE cp.role WHEN ? THEN 0 ELSE 1 END, lower(oi.display_name) ASC, oi.id ASC", domain.ConversationParticipantRoleOwner).
		Scan(ctx, &rows); err != nil {
		return GroupConversation{}, fmt.Errorf("list group conversation participants: %w", err)
	}
	participants := make([]GroupParticipant, 0, len(rows))
	for _, row := range rows {
		participants = append(participants, GroupParticipant{
			ChatSubjectID: row.ChatSubjectID,
			IdentityID:    row.IdentityID, DisplayName: row.DisplayName,
			AvatarFileID: row.AvatarFileID, Role: domain.ConversationParticipantRole(row.Role),
		})
	}
	return GroupConversation{
		ID: conversationID, Title: summary.Title, Description: summary.Description, ImageFileID: summary.ImageFileID,
		Status:    domain.ConversationStatus(summary.Status),
		CreatedAt: summary.CreatedAt, Participants: participants,
	}, nil
}

// Execute 在会话锁内幂等写入群聊文本消息。
func (a *SendGroupTextMessageAction) Execute(ctx context.Context, identity *servermodels.Identity, input GroupTextMessageInput) (ConversationMessage, error) {
	normalized, fields := normalizeGroupTextMessageInput(input)
	if len(fields) > 0 {
		return ConversationMessage{}, &ValidationError{Fields: fields}
	}
	messageID := uuid.NewV7()
	idempotencyKey := "mmsg:" + identity.OrganizationIdentity.ID + ":" + normalized.ClientMessageID
	var result ConversationMessage
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		sendContext, err := loadGroupSendContext(ctx, tx, identity, normalized.ConversationID)
		if err != nil {
			return err
		}
		if saved, found, err := loadIdempotentGroupMessage(ctx, tx, identity, normalized, idempotencyKey); err != nil || found {
			result = saved
			return err
		}
		reply, err := loadGroupReplyTarget(ctx, tx, identity.Organization.ID, normalized.ConversationID, normalized.ReplyToMessageID)
		if err != nil {
			return err
		}
		mentions, err := loadGroupMentionTargets(ctx, tx, identity.Organization.ID, normalized.ConversationID, sendContext.SubjectID, normalized.MentionSubjectIDs)
		if err != nil {
			return err
		}
		var replyToMessageID *string
		if reply != nil {
			replyToMessageID = &reply.ID
		}
		sequence, err := nextGroupMessageSequence(ctx, tx, identity.Organization.ID, normalized.ConversationID)
		if err != nil {
			return err
		}
		message := &servermodels.Message{
			GroupMessageSequence: &sequence,
			ID:                   messageID.String(), OrganizationID: identity.Organization.ID,
			ConversationID: normalized.ConversationID, SenderParticipantID: &sendContext.ParticipantID,
			Type: string(domain.MessageTypeText), Body: normalized.Body, ReplyToMessageID: replyToMessageID, MentionAll: normalized.MentionAll,
			IdempotencyKey: &idempotencyKey, OriginatedAt: time.Now().UTC(),
		}
		if _, err := tx.NewInsert().Model(message).
			Column("id", "organization_id", "conversation_id", "sender_participant_id", "type", "body", "reply_to_message_id", "mention_all", "idempotency_key", "originated_at", "group_message_sequence").
			Returning("*").
			Exec(ctx); err != nil {
			return fmt.Errorf("create group text message: %w", err)
		}
		if err := createMessageMentions(ctx, tx, identity.Organization.ID, message.ID, mentions); err != nil {
			return err
		}
		conversation := &servermodels.Conversation{ID: normalized.ConversationID, OrganizationID: identity.Organization.ID}
		if err := updateConversationSummary(ctx, tx, conversation, message); err != nil {
			return err
		}
		if err := advanceConversationUserReadState(ctx, tx, &servermodels.ConversationUserState{
			OrganizationID: identity.Organization.ID, ConversationID: normalized.ConversationID,
			UserID: identity.User.ID, LastReadMessageID: &message.ID,
		}, message); err != nil {
			return err
		}
		result = memberConversationMessage(message, sendContext.SubjectID, identity.OrganizationIdentity.ID, identity.OrganizationIdentity.DisplayName)
		result.ReplyTo = reply
		result.Mentions = mentions
		result.MentionAll = normalized.MentionAll
		return nil
	})
	if err != nil {
		return ConversationMessage{}, fmt.Errorf("send group message: %w", err)
	}
	return result, nil
}

// normalizeGroupTextMessageInput 规范化群聊文本、引用和提醒参数。
func normalizeGroupTextMessageInput(input GroupTextMessageInput) (GroupTextMessageInput, map[string]ValidationCode) {
	conversationID, clientMessageID, body, fields := normalizeInternalTextMessageInput(input.ConversationID, input.ClientMessageID, input.Body)
	input.ConversationID = conversationID
	input.ClientMessageID = clientMessageID
	input.Body = body
	if input.ReplyToMessageID != "" {
		var valid bool
		input.ReplyToMessageID, valid = common.NormalizeUUID(input.ReplyToMessageID)
		if !valid {
			fields["replyToMessageId"] = ValidationReplyToMessageIDInvalid
		}
	}
	if len(input.MentionSubjectIDs) > maxGroupParticipantCount-1 {
		fields["mentionSubjectIds"] = ValidationMentionSubjectIDsInvalid
	}
	seen := make(map[string]struct{}, len(input.MentionSubjectIDs))
	mentionSubjectIDs := make([]string, 0, len(input.MentionSubjectIDs))
	for _, subjectID := range input.MentionSubjectIDs {
		normalized, valid := common.NormalizeUUID(subjectID)
		if !valid {
			fields["mentionSubjectIds"] = ValidationMentionSubjectIDsInvalid
			continue
		}
		if _, duplicate := seen[normalized]; duplicate {
			fields["mentionSubjectIds"] = ValidationMentionSubjectIDsInvalid
			continue
		}
		seen[normalized] = struct{}{}
		mentionSubjectIDs = append(mentionSubjectIDs, normalized)
	}
	slices.Sort(mentionSubjectIDs)
	input.MentionSubjectIDs = mentionSubjectIDs
	return input, fields
}

// normalizeGroupConversationInput 规范化并校验群聊资料和初始成员。
func normalizeGroupConversationInput(currentIdentityID string, input GroupConversationInput) (GroupConversationInput, map[string]ValidationCode) {
	fields := map[string]ValidationCode{}
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.ImageFileID = strings.TrimSpace(input.ImageFileID)
	if input.Title == "" {
		fields["title"] = ValidationGroupTitleRequired
	} else if utf8.RuneCountInString(input.Title) > maxGroupTitleLength {
		fields["title"] = ValidationGroupTitleTooLong
	}
	if utf8.RuneCountInString(input.Description) > maxGroupDescriptionLength {
		fields["description"] = ValidationGroupDescriptionTooLong
	}
	if input.ImageFileID != "" {
		var valid bool
		input.ImageFileID, valid = common.NormalizeUUID(input.ImageFileID)
		if !valid {
			fields["imageFileId"] = ValidationGroupImageFileIDInvalid
		}
	}
	if len(input.MemberIdentityIDs) == 0 {
		fields["memberIdentityIds"] = ValidationGroupMembersRequired
	} else if len(input.MemberIdentityIDs)+1 > maxGroupParticipantCount {
		fields["memberIdentityIds"] = ValidationGroupMembersTooMany
	}
	seen := make(map[string]struct{}, len(input.MemberIdentityIDs))
	for index, identityID := range input.MemberIdentityIDs {
		normalized, valid := common.NormalizeUUID(identityID)
		if !valid || normalized == currentIdentityID {
			fields["memberIdentityIds"] = ValidationGroupMemberIDsInvalid
			continue
		}
		if _, duplicate := seen[normalized]; duplicate {
			fields["memberIdentityIds"] = ValidationGroupMemberIDsInvalid
			continue
		}
		seen[normalized] = struct{}{}
		input.MemberIdentityIDs[index] = normalized
	}
	return input, fields
}

// loadActiveGroupMembers 读取同企业可加入群聊的有效真人成员。
func loadActiveGroupMembers(ctx context.Context, db bun.IDB, organizationID string, identityIDs []string) ([]groupMemberRow, error) {
	rows := make([]groupMemberRow, 0, len(identityIDs))
	if err := db.NewSelect().
		TableExpr("organization_identities AS oi").
		ColumnExpr("oi.id AS identity_id").
		ColumnExpr("oi.display_name AS display_name").
		ColumnExpr("oi.avatar_file_id::text AS avatar_file_id").
		Join("JOIN users AS u ON u.organization_id = oi.organization_id AND u.identity_id = oi.id AND u.status = ?", domain.UserStatusActive).
		Where("oi.organization_id = ?", organizationID).
		Where("oi.type = ?", domain.OrganizationIdentityTypeUser).
		Where("oi.id IN (?)", bun.In(identityIDs)).
		OrderExpr("lower(oi.display_name) ASC, oi.id ASC").
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("load active group members: %w", err)
	}
	if len(rows) != len(identityIDs) {
		return nil, ErrGroupMemberNotFound
	}
	return rows, nil
}

// loadGroupSendContext 校验群聊及当前成员关系并返回发送上下文。
func loadGroupSendContext(ctx context.Context, db bun.IDB, identity *servermodels.Identity, conversationID string) (groupSendContextRow, error) {
	if _, err := lockConversationMember(ctx, db, identity, conversationID); err != nil {
		return groupSendContextRow{}, err
	}
	row := groupSendContextRow{}
	err := db.NewSelect().
		TableExpr("conversations AS cv").
		ColumnExpr("cv.id AS conversation_id").
		ColumnExpr("mine.id AS participant_id").
		ColumnExpr("mine.subject_id AS subject_id").
		Join("JOIN conversation_participants AS mine ON mine.organization_id = cv.organization_id AND mine.conversation_id = cv.id AND mine.left_at IS NULL").
		Join("JOIN chat_subjects AS mine_cs ON mine_cs.organization_id = mine.organization_id AND mine_cs.id = mine.subject_id AND mine_cs.kind = ? AND mine_cs.source_id = ?", domain.ChatSubjectKindOrganizationIdentity, identity.OrganizationIdentity.ID).
		Where("cv.organization_id = ?", identity.Organization.ID).
		Where("cv.id = ?", conversationID).
		Where("cv.type = ?", domain.ConversationTypeGroup).
		Where("cv.status = ?", domain.ConversationStatusActive).
		Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return groupSendContextRow{}, ErrConversationNotFound
	}
	if err != nil {
		return groupSendContextRow{}, fmt.Errorf("load group send context: %w", err)
	}
	return row, nil
}
