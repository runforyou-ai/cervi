//go:build server

package conversation

import (
	"context"
	"database/sql"
	"encoding/json"
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

var groupManagementRetryableConstraintNames = map[string]struct{}{
	"chat_subjects_organization_kind_source_unique": {},
}

// UpdateGroupConversationAction 修改群聊资料。
type UpdateGroupConversationAction struct{ db *bun.DB }

// AddGroupConversationMembersAction 批量增加群聊成员。
type AddGroupConversationMembersAction struct{ db *bun.DB }

// RemoveGroupConversationMemberAction 移除单个群聊成员。
type RemoveGroupConversationMemberAction struct{ db *bun.DB }

// TransferGroupConversationOwnerAction 转让群主。
type TransferGroupConversationOwnerAction struct{ db *bun.DB }

// LeaveGroupConversationAction 退出群聊，最后一位群主退出时解散群聊。
type LeaveGroupConversationAction struct{ db *bun.DB }

type lockedGroupConversationRow struct {
	Title                string  `bun:"title"`
	Description          string  `bun:"description"`
	ImageFileID          *string `bun:"image_file_id"`
	CurrentParticipantID string  `bun:"current_participant_id"`
	CurrentRole          string  `bun:"current_role"`
}

type activeGroupParticipantRow struct {
	ParticipantID string `bun:"participant_id"`
	IdentityID    string `bun:"identity_id"`
	DisplayName   string `bun:"display_name"`
	Role          string `bun:"role"`
}

// NewUpdateGroupConversationAction 创建群聊资料修改操作。
func NewUpdateGroupConversationAction(db *bun.DB) *UpdateGroupConversationAction {
	return &UpdateGroupConversationAction{db: db}
}

// NewAddGroupConversationMembersAction 创建群聊增员操作。
func NewAddGroupConversationMembersAction(db *bun.DB) *AddGroupConversationMembersAction {
	return &AddGroupConversationMembersAction{db: db}
}

// NewRemoveGroupConversationMemberAction 创建群聊成员移除操作。
func NewRemoveGroupConversationMemberAction(db *bun.DB) *RemoveGroupConversationMemberAction {
	return &RemoveGroupConversationMemberAction{db: db}
}

// NewTransferGroupConversationOwnerAction 创建群主转让操作。
func NewTransferGroupConversationOwnerAction(db *bun.DB) *TransferGroupConversationOwnerAction {
	return &TransferGroupConversationOwnerAction{db: db}
}

// NewLeaveGroupConversationAction 创建群聊退出操作。
func NewLeaveGroupConversationAction(db *bun.DB) *LeaveGroupConversationAction {
	return &LeaveGroupConversationAction{db: db}
}

// Execute 修改群聊资料，并在名称变化时记录系统事件。
func (a *UpdateGroupConversationAction) Execute(ctx context.Context, identity *servermodels.Identity, input GroupConversationProfileInput) (GroupConversation, error) {
	normalized, fields := normalizeGroupProfileInput(input)
	if len(fields) > 0 {
		return GroupConversation{}, &ValidationError{Fields: fields}
	}
	var result GroupConversation
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		group, err := lockGroupConversation(ctx, tx, identity, normalized.ConversationID)
		if err != nil {
			return err
		}
		if err := requireGroupOwner(group); err != nil {
			return err
		}
		nextImageFileID := group.ImageFileID
		imageChanged := false
		if normalized.ImageFileID != nil {
			nextImageFileID, err = activateGroupImage(ctx, tx, identity.Organization.ID, *normalized.ImageFileID, group.ImageFileID)
			if err != nil {
				return err
			}
			imageChanged = group.ImageFileID == nil || *group.ImageFileID != *normalized.ImageFileID
		}
		if group.Title != normalized.Title || group.Description != normalized.Description || imageChanged {
			if _, err := tx.NewUpdate().Model((*servermodels.Conversation)(nil)).
				Set("title = ?", normalized.Title).
				Set("description = ?", common.OptionalString(normalized.Description)).
				Set("image_file_id = ?", nextImageFileID).
				Set("updated_at = now()").
				Where("organization_id = ?", identity.Organization.ID).
				Where("id = ?", normalized.ConversationID).
				Exec(ctx); err != nil {
				return fmt.Errorf("update group conversation profile: %w", err)
			}
			if imageChanged {
				if err := retireGroupImage(ctx, tx, identity.Organization.ID, group.ImageFileID, nextImageFileID); err != nil {
					return err
				}
			}
		}
		if group.Title != normalized.Title {
			previousTitle := group.Title
			eventTitle := normalized.Title
			if err := createGroupSystemEvent(ctx, tx, identity, normalized.ConversationID, ConversationSystemEvent{
				Type:          domain.ConversationSystemEventGroupRenamed,
				Actor:         groupActorSnapshot(identity),
				PreviousTitle: &previousTitle,
				Title:         &eventTitle,
			}); err != nil {
				return err
			}
		}
		result, err = loadGroupConversation(ctx, tx, identity, normalized.ConversationID)
		return err
	})
	if err != nil {
		return GroupConversation{}, fmt.Errorf("update group conversation: %w", err)
	}
	return result, nil
}

// Execute 增加有效真人成员，重新加入时复用原参与者行。
func (a *AddGroupConversationMembersAction) Execute(ctx context.Context, identity *servermodels.Identity, input GroupConversationMembersInput) (GroupConversation, error) {
	conversationID, memberIDs, fields := normalizeGroupMembersInput(identity.OrganizationIdentity.ID, input.ConversationID, input.MemberIdentityIDs)
	if len(fields) > 0 {
		return GroupConversation{}, &ValidationError{Fields: fields}
	}
	participantIDs := make(map[string]string, len(memberIDs))
	subjectIDs := make(map[string]string, len(memberIDs))
	for _, identityID := range memberIDs {
		participantIDs[identityID] = uuid.NewV7().String()
		subjectIDs[identityID] = uuid.NewV7().String()
	}

	var result GroupConversation
	var err error
	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
				return err
			}
			group, err := lockGroupConversation(ctx, tx, identity, conversationID)
			if err != nil {
				return err
			}
			if err := requireGroupOwner(group); err != nil {
				return err
			}
			members, err := loadActiveGroupMembers(ctx, tx, identity.Organization.ID, memberIDs)
			if err != nil {
				return err
			}
			activeIDs, err := loadActiveGroupParticipantIdentityIDs(ctx, tx, identity.Organization.ID, conversationID)
			if err != nil {
				return err
			}
			activeSet := make(map[string]struct{}, len(activeIDs))
			for _, identityID := range activeIDs {
				activeSet[identityID] = struct{}{}
			}
			for _, identityID := range memberIDs {
				if _, exists := activeSet[identityID]; exists {
					return &ConflictError{Reason: ConflictReasonGroupMemberAlreadyActive}
				}
			}
			if len(activeIDs)+len(memberIDs) > maxGroupParticipantCount {
				return &ValidationError{Fields: map[string]ValidationCode{"memberIdentityIds": ValidationGroupMembersTooMany}}
			}

			targets := make([]ConversationSystemEventParticipant, 0, len(members))
			for _, member := range members {
				subject, err := ensureOrganizationIdentityChatSubject(ctx, tx, identity.Organization.ID, member.IdentityID, subjectIDs[member.IdentityID])
				if err != nil {
					return err
				}
				if err := restoreOrCreateGroupParticipant(ctx, tx, identity.Organization.ID, conversationID, subject.ID, participantIDs[member.IdentityID]); err != nil {
					return err
				}
				targets = append(targets, ConversationSystemEventParticipant{IdentityID: member.IdentityID, DisplayName: member.DisplayName})
			}
			if err := createGroupSystemEvent(ctx, tx, identity, conversationID, ConversationSystemEvent{
				Type: domain.ConversationSystemEventGroupMembersAdded, Actor: groupActorSnapshot(identity), Targets: targets,
			}); err != nil {
				return err
			}
			// 新成员从本轮加入事件开始记录已读，离开期间的历史不形成未读。
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO conversation_user_states (organization_id, conversation_id, user_id, last_read_message_id)
				SELECT u.organization_id, cv.id, u.id, cv.last_message_id
				FROM users AS u
				JOIN conversations AS cv ON cv.organization_id = u.organization_id AND cv.id = ?
				WHERE u.organization_id = ? AND u.identity_id IN (?)
				ON CONFLICT (organization_id, conversation_id, user_id) DO UPDATE
				SET last_read_message_id = EXCLUDED.last_read_message_id, last_read_at = now(), updated_at = now()
			`, conversationID, identity.Organization.ID, bun.In(memberIDs)); err != nil {
				return fmt.Errorf("initialize added group member read states: %w", err)
			}
			result, err = loadGroupConversation(ctx, tx, identity, conversationID)
			return err
		})
		if err == nil {
			return result, nil
		}
		constraint, retryable := retryableUniqueViolation(err, groupManagementRetryableConstraintNames)
		if !retryable {
			return GroupConversation{}, fmt.Errorf("add group conversation members: %w", err)
		}
		if attempt < maxWriteAttempts-1 {
			slog.Info("企业群聊增员重试", "conversation_id", conversationID, "attempt", attempt+2, "constraint", constraint)
		}
	}
	return GroupConversation{}, fmt.Errorf("add group conversation members retries exhausted: %w", err)
}

// Execute 将当前有效的普通成员移出群聊。
func (a *RemoveGroupConversationMemberAction) Execute(ctx context.Context, identity *servermodels.Identity, input GroupConversationMemberInput) (GroupConversation, error) {
	conversationID, memberID, fields := normalizeGroupMemberInput(input.ConversationID, input.MemberIdentityID, "memberIdentityId", ValidationGroupMemberIDInvalid)
	if len(fields) > 0 {
		return GroupConversation{}, &ValidationError{Fields: fields}
	}
	var result GroupConversation
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		group, err := lockGroupConversation(ctx, tx, identity, conversationID)
		if err != nil {
			return err
		}
		if err := requireGroupOwner(group); err != nil {
			return err
		}
		target, err := loadActiveGroupParticipant(ctx, tx, identity.Organization.ID, conversationID, memberID, false)
		if err != nil {
			return err
		}
		if target.Role == string(domain.ConversationParticipantRoleOwner) {
			return &ConflictError{Reason: ConflictReasonGroupOwnerCannotBeRemoved}
		}
		if err := leaveGroupParticipant(ctx, tx, identity.Organization.ID, target.ParticipantID); err != nil {
			return err
		}
		if err := createGroupSystemEvent(ctx, tx, identity, conversationID, ConversationSystemEvent{
			Type: domain.ConversationSystemEventGroupMemberRemoved, Actor: groupActorSnapshot(identity), Targets: []ConversationSystemEventParticipant{groupParticipantSnapshot(target)},
		}); err != nil {
			return err
		}
		result, err = loadGroupConversation(ctx, tx, identity, conversationID)
		return err
	})
	if err != nil {
		return GroupConversation{}, fmt.Errorf("remove group conversation member: %w", err)
	}
	return result, nil
}

// Execute 将群主角色转让给另一位当前成员。
func (a *TransferGroupConversationOwnerAction) Execute(ctx context.Context, identity *servermodels.Identity, input GroupConversationOwnerInput) (GroupConversation, error) {
	conversationID, ownerID, fields := normalizeGroupMemberInput(input.ConversationID, input.OwnerIdentityID, "ownerIdentityId", ValidationGroupOwnerIDInvalid)
	if len(fields) > 0 {
		return GroupConversation{}, &ValidationError{Fields: fields}
	}
	var result GroupConversation
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		group, err := lockGroupConversation(ctx, tx, identity, conversationID)
		if err != nil {
			return err
		}
		if err := requireGroupOwner(group); err != nil {
			return err
		}
		if ownerID == identity.OrganizationIdentity.ID {
			result, err = loadGroupConversation(ctx, tx, identity, conversationID)
			return err
		}
		target, err := loadActiveGroupParticipant(ctx, tx, identity.Organization.ID, conversationID, ownerID, true)
		if err != nil {
			return err
		}
		if err := transferGroupOwner(ctx, tx, identity.Organization.ID, group.CurrentParticipantID, target.ParticipantID); err != nil {
			return err
		}
		if err := createGroupSystemEvent(ctx, tx, identity, conversationID, ConversationSystemEvent{
			Type: domain.ConversationSystemEventGroupOwnerTransferred, Actor: groupActorSnapshot(identity), Targets: []ConversationSystemEventParticipant{groupParticipantSnapshot(target)},
		}); err != nil {
			return err
		}
		result, err = loadGroupConversation(ctx, tx, identity, conversationID)
		return err
	})
	if err != nil {
		return GroupConversation{}, fmt.Errorf("transfer group conversation owner: %w", err)
	}
	return result, nil
}

// Execute 退出群聊，群主退出时转让群主或解散只有自己的群聊。
func (a *LeaveGroupConversationAction) Execute(ctx context.Context, identity *servermodels.Identity, input GroupConversationLeaveInput) error {
	conversationID, valid := common.NormalizeUUID(input.ConversationID)
	fields := map[string]ValidationCode{}
	if !valid {
		fields["conversationId"] = ValidationConversationIDInvalid
	}
	successorID := strings.TrimSpace(input.SuccessorIdentityID)
	if successorID != "" {
		var successorValid bool
		successorID, successorValid = common.NormalizeUUID(successorID)
		if !successorValid || successorID == identity.OrganizationIdentity.ID {
			fields["successorIdentityId"] = ValidationGroupSuccessorIDInvalid
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}

	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		group, err := lockGroupConversation(ctx, tx, identity, conversationID)
		if err != nil {
			return err
		}
		if group.CurrentRole == string(domain.ConversationParticipantRoleOwner) {
			if successorID == "" {
				activeIdentityIDs, err := loadActiveGroupParticipantIdentityIDs(ctx, tx, identity.Organization.ID, conversationID)
				if err != nil {
					return err
				}
				if len(activeIdentityIDs) != 1 {
					return &ConflictError{Reason: ConflictReasonGroupSuccessorRequired}
				}
				if err := createGroupSystemEvent(ctx, tx, identity, conversationID, ConversationSystemEvent{
					Type: domain.ConversationSystemEventGroupDissolved, Actor: groupActorSnapshot(identity),
				}); err != nil {
					return err
				}
				if err := archiveGroupConversation(ctx, tx, identity.Organization.ID, conversationID); err != nil {
					return err
				}
				return nil
			}
			target, err := loadActiveGroupParticipant(ctx, tx, identity.Organization.ID, conversationID, successorID, true)
			if err != nil {
				return err
			}
			if err := transferGroupOwner(ctx, tx, identity.Organization.ID, group.CurrentParticipantID, target.ParticipantID); err != nil {
				return err
			}
			if err := createGroupSystemEvent(ctx, tx, identity, conversationID, ConversationSystemEvent{
				Type: domain.ConversationSystemEventGroupOwnerTransferred, Actor: groupActorSnapshot(identity), Targets: []ConversationSystemEventParticipant{groupParticipantSnapshot(target)},
			}); err != nil {
				return err
			}
		} else if successorID != "" {
			return &ValidationError{Fields: map[string]ValidationCode{"successorIdentityId": ValidationGroupSuccessorIDInvalid}}
		}
		if err := leaveGroupParticipant(ctx, tx, identity.Organization.ID, group.CurrentParticipantID); err != nil {
			return err
		}
		return createGroupSystemEvent(ctx, tx, identity, conversationID, ConversationSystemEvent{
			Type: domain.ConversationSystemEventGroupMemberLeft, Actor: groupActorSnapshot(identity),
		})
	})
	if err != nil {
		return fmt.Errorf("leave group conversation: %w", err)
	}
	return nil
}

// normalizeGroupProfileInput 规范化群聊资料修改参数。
func normalizeGroupProfileInput(input GroupConversationProfileInput) (GroupConversationProfileInput, map[string]ValidationCode) {
	fields := map[string]ValidationCode{}
	normalizedConversationID, valid := common.NormalizeUUID(input.ConversationID)
	if !valid {
		fields["conversationId"] = ValidationConversationIDInvalid
	}
	input.ConversationID = normalizedConversationID
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	if input.Title == "" {
		fields["title"] = ValidationGroupTitleRequired
	} else if utf8.RuneCountInString(input.Title) > maxGroupTitleLength {
		fields["title"] = ValidationGroupTitleTooLong
	}
	if utf8.RuneCountInString(input.Description) > maxGroupDescriptionLength {
		fields["description"] = ValidationGroupDescriptionTooLong
	}
	if input.ImageFileID != nil {
		imageFileID, imageFileIDValid := common.NormalizeUUID(strings.TrimSpace(*input.ImageFileID))
		if !imageFileIDValid {
			fields["imageFileId"] = ValidationGroupImageFileIDInvalid
		}
		input.ImageFileID = &imageFileID
	}
	return input, fields
}

// normalizeGroupMembersInput 规范化群聊批量增员参数。
func normalizeGroupMembersInput(currentIdentityID, conversationID string, memberIDs []string) (string, []string, map[string]ValidationCode) {
	fields := map[string]ValidationCode{}
	normalizedConversationID, valid := common.NormalizeUUID(conversationID)
	if !valid {
		fields["conversationId"] = ValidationConversationIDInvalid
	}
	if len(memberIDs) == 0 {
		fields["memberIdentityIds"] = ValidationGroupMembersRequired
	} else if len(memberIDs) > maxGroupParticipantCount-1 {
		fields["memberIdentityIds"] = ValidationGroupMembersTooMany
	}
	seen := make(map[string]struct{}, len(memberIDs))
	normalizedIDs := make([]string, 0, len(memberIDs))
	for _, identityID := range memberIDs {
		identityID, valid = common.NormalizeUUID(identityID)
		if !valid || identityID == currentIdentityID {
			fields["memberIdentityIds"] = ValidationGroupMemberIDsInvalid
			continue
		}
		if _, exists := seen[identityID]; exists {
			fields["memberIdentityIds"] = ValidationGroupMemberIDsInvalid
			continue
		}
		seen[identityID] = struct{}{}
		normalizedIDs = append(normalizedIDs, identityID)
	}
	return normalizedConversationID, normalizedIDs, fields
}

// normalizeGroupMemberInput 规范化群聊单成员操作参数。
func normalizeGroupMemberInput(conversationID, identityID, field string, code ValidationCode) (string, string, map[string]ValidationCode) {
	fields := map[string]ValidationCode{}
	normalizedConversationID, valid := common.NormalizeUUID(conversationID)
	if !valid {
		fields["conversationId"] = ValidationConversationIDInvalid
	}
	normalizedIdentityID, valid := common.NormalizeUUID(identityID)
	if !valid {
		fields[field] = code
	}
	return normalizedConversationID, normalizedIdentityID, fields
}

// lockGroupConversation 锁定当前成员可见的有效群聊。
func lockGroupConversation(ctx context.Context, db bun.IDB, identity *servermodels.Identity, conversationID string) (lockedGroupConversationRow, error) {
	row := lockedGroupConversationRow{}
	err := db.NewSelect().
		TableExpr("conversations AS cv").
		ColumnExpr("cv.title AS title").
		ColumnExpr("COALESCE(cv.description, '') AS description").
		ColumnExpr("cv.image_file_id::text AS image_file_id").
		ColumnExpr("mine.id AS current_participant_id").
		ColumnExpr("mine.role AS current_role").
		Join("JOIN conversation_participants AS mine ON mine.organization_id = cv.organization_id AND mine.conversation_id = cv.id AND mine.left_at IS NULL").
		Join("JOIN chat_subjects AS mine_cs ON mine_cs.organization_id = mine.organization_id AND mine_cs.id = mine.subject_id AND mine_cs.kind = ? AND mine_cs.source_id = ?", domain.ChatSubjectKindOrganizationIdentity, identity.OrganizationIdentity.ID).
		Where("cv.organization_id = ?", identity.Organization.ID).
		Where("cv.id = ?", conversationID).
		Where("cv.type = ?", domain.ConversationTypeGroup).
		Where("cv.status = ?", domain.ConversationStatusActive).
		For("UPDATE OF cv").
		Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return lockedGroupConversationRow{}, ErrConversationNotFound
	}
	if err != nil {
		return lockedGroupConversationRow{}, fmt.Errorf("lock group conversation: %w", err)
	}
	return row, nil
}

// requireGroupOwner 校验当前成员持有群主角色。
func requireGroupOwner(group lockedGroupConversationRow) error {
	if group.CurrentRole != string(domain.ConversationParticipantRoleOwner) {
		return ErrGroupOwnerRequired
	}
	return nil
}

// loadActiveGroupParticipantIdentityIDs 读取群聊当前有效成员编号。
func loadActiveGroupParticipantIdentityIDs(ctx context.Context, db bun.IDB, organizationID, conversationID string) ([]string, error) {
	identityIDs := make([]string, 0)
	if err := db.NewSelect().
		TableExpr("conversation_participants AS cp").
		ColumnExpr("cs.source_id").
		Join("JOIN chat_subjects AS cs ON cs.organization_id = cp.organization_id AND cs.id = cp.subject_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Where("cp.organization_id = ?", organizationID).
		Where("cp.conversation_id = ?", conversationID).
		Where("cp.left_at IS NULL").
		OrderExpr("cs.source_id ASC").
		Scan(ctx, &identityIDs); err != nil {
		return nil, fmt.Errorf("load active group participant identity ids: %w", err)
	}
	return identityIDs, nil
}

// loadActiveGroupParticipant 锁定指定的当前有效群成员。
func loadActiveGroupParticipant(ctx context.Context, db bun.IDB, organizationID, conversationID, identityID string, requireActiveUser bool) (activeGroupParticipantRow, error) {
	row := activeGroupParticipantRow{}
	query := db.NewSelect().
		TableExpr("conversation_participants AS cp").
		ColumnExpr("cp.id AS participant_id").
		ColumnExpr("cs.source_id AS identity_id").
		ColumnExpr("oi.display_name AS display_name").
		ColumnExpr("cp.role AS role").
		Join("JOIN chat_subjects AS cs ON cs.organization_id = cp.organization_id AND cs.id = cp.subject_id AND cs.kind = ? AND cs.source_id = ?", domain.ChatSubjectKindOrganizationIdentity, identityID).
		Join("JOIN organization_identities AS oi ON oi.organization_id = cs.organization_id AND oi.id = cs.source_id")
	if requireActiveUser {
		query = query.Join("JOIN users AS u ON u.organization_id = oi.organization_id AND u.identity_id = oi.id AND u.status = ?", domain.UserStatusActive)
	}
	err := query.
		Where("cp.organization_id = ?", organizationID).
		Where("cp.conversation_id = ?", conversationID).
		Where("cp.left_at IS NULL").
		For("UPDATE OF cp").
		Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return activeGroupParticipantRow{}, &ConflictError{Reason: ConflictReasonGroupMemberNotActive}
	}
	if err != nil {
		return activeGroupParticipantRow{}, fmt.Errorf("load active group participant: %w", err)
	}
	return row, nil
}

// restoreOrCreateGroupParticipant 复用退出成员关系或建立新的成员关系。
func restoreOrCreateGroupParticipant(ctx context.Context, db bun.IDB, organizationID, conversationID, subjectID, participantID string) error {
	participant := &servermodels.ConversationParticipant{}
	err := db.NewSelect().Model(participant).
		Where("cp.organization_id = ?", organizationID).
		Where("cp.conversation_id = ?", conversationID).
		Where("cp.subject_id = ?", subjectID).
		For("UPDATE").
		Scan(ctx)
	if err == nil {
		if participant.LeftAt == nil {
			return &ConflictError{Reason: ConflictReasonGroupMemberAlreadyActive}
		}
		if _, err := db.NewUpdate().Model(participant).
			Set("left_at = NULL").
			Set("role = ?", domain.ConversationParticipantRoleMember).
			Set("updated_at = now()").
			WherePK().
			Where("organization_id = ?", organizationID).
			Exec(ctx); err != nil {
			return fmt.Errorf("restore group conversation participant: %w", err)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("find group conversation participant: %w", err)
	}
	participant = &servermodels.ConversationParticipant{
		ID: participantID, OrganizationID: organizationID, ConversationID: conversationID,
		SubjectID: subjectID, Role: string(domain.ConversationParticipantRoleMember),
	}
	if _, err := db.NewInsert().Model(participant).
		Column("id", "organization_id", "conversation_id", "subject_id", "role").
		Exec(ctx); err != nil {
		return fmt.Errorf("create group conversation participant: %w", err)
	}
	return nil
}

// transferGroupOwner 在已锁定群聊中原子切换唯一群主。
func transferGroupOwner(ctx context.Context, db bun.IDB, organizationID, currentParticipantID, successorParticipantID string) error {
	if _, err := db.NewUpdate().Model((*servermodels.ConversationParticipant)(nil)).
		Set("role = CASE WHEN id = ? THEN ? ELSE ? END", successorParticipantID, domain.ConversationParticipantRoleOwner, domain.ConversationParticipantRoleMember).
		Set("updated_at = now()").
		Where("organization_id = ?", organizationID).
		Where("id IN (?)", bun.In([]string{currentParticipantID, successorParticipantID})).
		Exec(ctx); err != nil {
		return fmt.Errorf("transfer group conversation owner: %w", err)
	}
	return nil
}

// leaveGroupParticipant 标记成员已经退出群聊。
func leaveGroupParticipant(ctx context.Context, db bun.IDB, organizationID, participantID string) error {
	if _, err := db.NewUpdate().Model((*servermodels.ConversationParticipant)(nil)).
		Set("left_at = now()").
		Set("role = ?", domain.ConversationParticipantRoleMember).
		Set("updated_at = now()").
		Where("organization_id = ?", organizationID).
		Where("id = ?", participantID).
		Where("left_at IS NULL").
		Exec(ctx); err != nil {
		return fmt.Errorf("leave group conversation participant: %w", err)
	}
	return nil
}

// archiveGroupConversation 归档已经由最后一位成员解散的群聊。
func archiveGroupConversation(ctx context.Context, db bun.IDB, organizationID, conversationID string) error {
	if _, err := db.NewUpdate().Model((*servermodels.Conversation)(nil)).
		Set("status = ?", domain.ConversationStatusArchived).
		Set("updated_at = now()").
		Where("organization_id = ?", organizationID).
		Where("id = ?", conversationID).
		Where("type = ?", domain.ConversationTypeGroup).
		Where("status = ?", domain.ConversationStatusActive).
		Exec(ctx); err != nil {
		return fmt.Errorf("archive dissolved group conversation: %w", err)
	}
	return nil
}

// createGroupSystemEvent 写入类型化系统事件并推进会话摘要。
func createGroupSystemEvent(ctx context.Context, db bun.IDB, identity *servermodels.Identity, conversationID string, event ConversationSystemEvent) error {
	if event.Targets == nil {
		event.Targets = make([]ConversationSystemEventParticipant, 0)
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal group system event: %w", err)
	}
	eventType := string(event.Type)
	message := &servermodels.Message{
		ID: uuid.NewV7().String(), OrganizationID: identity.Organization.ID, ConversationID: conversationID,
		Type: string(domain.MessageTypeSystem), Body: "", SystemEventType: &eventType, SystemEventPayload: payload,
		OriginatedAt: time.Now().UTC(),
	}
	if _, err := db.NewInsert().Model(message).
		Column("id", "organization_id", "conversation_id", "type", "body", "system_event_type", "system_event_payload", "originated_at").
		Exec(ctx); err != nil {
		return fmt.Errorf("create group system event: %w", err)
	}
	conversation := &servermodels.Conversation{ID: conversationID, OrganizationID: identity.Organization.ID}
	if err := updateConversationSummary(ctx, db, conversation, message); err != nil {
		return err
	}
	state := &servermodels.ConversationUserState{
		OrganizationID: identity.Organization.ID, ConversationID: conversationID,
		UserID: identity.User.ID, LastReadMessageID: message.ID,
	}
	if err := advanceConversationUserReadState(ctx, db, state, message); err != nil {
		return err
	}
	return nil
}

// groupActorSnapshot 记录操作人的审计快照。
func groupActorSnapshot(identity *servermodels.Identity) ConversationSystemEventParticipant {
	return ConversationSystemEventParticipant{IdentityID: identity.OrganizationIdentity.ID, DisplayName: identity.OrganizationIdentity.DisplayName}
}

// groupParticipantSnapshot 记录目标成员的审计快照。
func groupParticipantSnapshot(participant activeGroupParticipantRow) ConversationSystemEventParticipant {
	return ConversationSystemEventParticipant{IdentityID: participant.IdentityID, DisplayName: participant.DisplayName}
}
