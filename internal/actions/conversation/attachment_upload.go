//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
	"slices"
)

// UpdateUploads 确认上传活跃、失败、完成或取消，并保持已入库消息的位置不变。
func (a *SendAttachmentMessageAction) UpdateUploads(ctx context.Context, identity *servermodels.Identity, fileIDs []string, status domain.AttachmentUploadStatus) error {
	if len(fileIDs) == 0 || len(fileIDs) > 100 {
		return fileaction.ErrFileNotFound
	}
	if status != domain.AttachmentUploading && status != domain.AttachmentReady && status != domain.AttachmentFailed && status != domain.AttachmentCancelled {
		return fileaction.ErrFileNotFound
	}
	ids := slices.Clone(fileIDs)
	slices.Sort(ids)
	for _, id := range ids {
		if !common.ValidUUID(id) {
			return fileaction.ErrFileNotFound
		}
	}
	return a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		agentMembers := make([]chatstate.Member, 0)
		for _, id := range ids {
			member, err := updateAttachmentUpload(ctx, tx, identity, id, status)
			if err != nil {
				return err
			}
			// 按首次出现顺序收集涉及的 AI 聊天，更新全部附件后再判断成员是否已无上传中的附件。
			if member.Conversation.Type == string(domain.ConversationTypeAgent) &&
				!slices.ContainsFunc(agentMembers, func(item chatstate.Member) bool { return item.Conversation.ID == member.Conversation.ID }) {
				agentMembers = append(agentMembers, member)
			}
		}
		for _, member := range agentMembers {
			if err := a.scheduleCompletedAttachments(ctx, tx, identity, member); err != nil {
				return err
			}
		}
		return nil
	})
}

// scheduleCompletedAttachments 在 AI 聊天仍可发送且成员已无上传中的附件时，以最新一条晚于已有输入来源的已完成附件追加一次 Agent 输入。
func (a *SendAttachmentMessageAction) scheduleCompletedAttachments(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, member chatstate.Member) error {
	sendContext, err := lockAgentSendContext(ctx, tx, identity, member.Conversation.ID)
	if errors.Is(err, ErrConversationNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	uploading, err := tx.NewSelect().TableExpr("message_attachments AS ma").
		Join("JOIN messages AS msg ON msg.id = ma.message_id AND msg.organization_id = ma.organization_id").
		Where("msg.organization_id = ? AND msg.conversation_id = ? AND msg.sender_participant_id = ?", identity.Organization.ID, member.Conversation.ID, member.ParticipantID).
		Where("msg.deleted_at IS NULL AND ma.upload_status = ? AND ma.upload_expires_at > now()", domain.AttachmentUploading).
		Exists(ctx)
	if err != nil || uploading {
		return err
	}
	// 输入来源取晚于该 Agent 已有输入来源的最新已完成附件，保证输入序号与消息顺序一致。
	var messageID string
	err = tx.NewRaw(`
		SELECT msg.id
		FROM messages AS msg
		JOIN message_attachments AS ma ON ma.message_id = msg.id AND ma.organization_id = msg.organization_id
		WHERE msg.organization_id = ? AND msg.conversation_id = ? AND msg.sender_participant_id = ?
			AND msg.type = ? AND msg.deleted_at IS NULL AND ma.upload_status = ?
			AND msg.message_seq > COALESCE((
				SELECT MAX(source.message_seq)
				FROM agent_lanes AS al
				JOIN agent_inputs AS ai ON ai.lane_id = al.id
				JOIN messages AS source ON source.id = ai.source_message_id AND source.organization_id = ai.organization_id
				WHERE al.organization_id = msg.organization_id AND al.scope_kind = ?
					AND al.scope_id = msg.conversation_id AND al.agent_identity_id = ?
			), 0)
		ORDER BY msg.message_seq DESC
		LIMIT 1
	`, identity.Organization.ID, member.Conversation.ID, sendContext.ParticipantID, domain.MessageTypeAttachment, domain.AttachmentReady,
		domain.AgentExecutionScopeConversation, sendContext.AgentIdentityID).Scan(ctx, &messageID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load completed AI chat attachment: %w", err)
	}
	if a.scheduler == nil || sendContext.AgentRevisionID == nil {
		return ErrDataInvariant
	}
	if err := a.scheduler.Schedule(ctx, tx, identity.Organization.ID, member.Conversation.ID, sendContext.AgentIdentityID, *sendContext.AgentRevisionID, messageID, sendContext.SubjectID); err != nil {
		return fmt.Errorf("schedule AI chat attachments: %w", err)
	}
	return nil
}

// updateAttachmentUpload 在会话和文件锁内推进一条附件消息的内容状态，并返回已锁定的发送成员。
func updateAttachmentUpload(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, fileID string, status domain.AttachmentUploadStatus) (chatstate.Member, error) {
	row := struct {
		MessageID      string
		ConversationID string
	}{}
	err := tx.NewSelect().TableExpr("message_attachments AS ma").ColumnExpr("ma.message_id, msg.conversation_id").
		Join("JOIN messages msg ON msg.id = ma.message_id AND msg.organization_id = ma.organization_id").
		Join("JOIN conversation_participants cp ON cp.id = msg.sender_participant_id AND cp.organization_id = msg.organization_id").
		Join("JOIN chat_subjects cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id").
		Where("ma.organization_id = ? AND ma.file_id = ? AND cs.kind = ? AND cs.source_id = ?", identity.Organization.ID, fileID, domain.ChatSubjectKindOrganizationIdentity, identity.OrganizationIdentity.ID).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return chatstate.Member{}, fileaction.ErrFileNotFound
	}
	if err != nil {
		return chatstate.Member{}, err
	}
	member, err := chatstate.LockMember(ctx, tx, identity, row.ConversationID)
	if err != nil {
		return member, err
	}
	if member.Conversation.Type != string(domain.ConversationTypeDirect) && member.Conversation.Type != string(domain.ConversationTypeAgent) {
		return member, ErrConversationNotFound
	}
	file := &servermodels.File{}
	if err := tx.NewSelect().Model(file).Where("f.id = ? AND f.organization_id = ? AND f.created_by_user_id = ?", fileID, identity.Organization.ID, identity.User.ID).For("UPDATE").Scan(ctx); err != nil {
		return member, err
	}
	var current domain.AttachmentUploadStatus
	if err := tx.NewSelect().Table("message_attachments").Column("upload_status").Where("message_id = ?", row.MessageID).Scan(ctx, &current); err != nil {
		return member, err
	}
	// 并发取消与完成按取消结果收敛，已取消消息保持取消状态。
	if current == domain.AttachmentCancelled || (current == domain.AttachmentReady && status != domain.AttachmentCancelled) {
		return member, nil
	}
	if file.Status == string(domain.FileStatusDeleting) {
		return member, fileaction.ErrFileNotFound
	}
	switch status {
	case domain.AttachmentReady:
		if file.Status != string(domain.FileStatusUploaded) {
			return member, fileaction.ErrFileNotFound
		}
		if _, err := tx.NewUpdate().Model(file).Set("status = ?", domain.FileStatusActive).Set("expires_at = NULL").Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
			return member, err
		}
	case domain.AttachmentUploading:
		if _, err := tx.NewUpdate().Model(file).Set("expires_at = now() + interval '24 hours'").Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
			return member, err
		}
	case domain.AttachmentCancelled:
		if _, err := tx.NewUpdate().Model(file).Set("status = ?", domain.FileStatusDeleting).Set("expires_at = now()").Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
			return member, err
		}
		if _, err := tx.NewUpdate().Model((*servermodels.Message)(nil)).Set("deleted_at = now()").Set("updated_at = now()").Where("id = ?", row.MessageID).Exec(ctx); err != nil {
			return member, err
		}
		if err := chatstate.RecomputeConversationSummary(ctx, tx, member.Conversation, row.MessageID); err != nil {
			return member, err
		}
	}
	// 附件完成或取消时推进会话版本。
	if status == domain.AttachmentReady || status == domain.AttachmentCancelled {
		if _, err := tx.NewUpdate().Model(member.Conversation).Set("version = version + 1").WherePK().Exec(ctx); err != nil {
			return err
		}
	}
	_, err = tx.NewRaw(`UPDATE message_attachments SET upload_status = ?, upload_expires_at = CASE WHEN ? = 'uploading' THEN now() + interval '2 minutes' END WHERE message_id = ?`, status, status, row.MessageID).Exec(ctx)
	if err != nil {
		return member, fmt.Errorf("update attachment upload: %w", err)
	}
	return member, nil
}
