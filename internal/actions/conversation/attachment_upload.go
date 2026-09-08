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
		for _, id := range ids {
			if err := updateAttachmentUpload(ctx, tx, identity, id, status); err != nil {
				return err
			}
		}
		return nil
	})
}

// updateAttachmentUpload 在会话和文件锁内推进一条附件消息的内容状态。
func updateAttachmentUpload(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, fileID string, status domain.AttachmentUploadStatus) error {
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
		return fileaction.ErrFileNotFound
	}
	if err != nil {
		return err
	}
	member, err := chatstate.LockMember(ctx, tx, identity, row.ConversationID)
	if err != nil {
		return err
	}
	if member.Conversation.Type != string(domain.ConversationTypeDirect) {
		return ErrConversationNotFound
	}
	file := &servermodels.File{}
	if err := tx.NewSelect().Model(file).Where("f.id = ? AND f.organization_id = ? AND f.created_by_user_id = ?", fileID, identity.Organization.ID, identity.User.ID).For("UPDATE").Scan(ctx); err != nil {
		return err
	}
	var current domain.AttachmentUploadStatus
	if err := tx.NewSelect().Table("message_attachments").Column("upload_status").Where("message_id = ?", row.MessageID).Scan(ctx, &current); err != nil {
		return err
	}
	// 取消优先于并发完成，迟到的进度或完成请求不能恢复已取消的消息。
	if current == domain.AttachmentCancelled || (current == domain.AttachmentReady && status != domain.AttachmentCancelled) {
		return nil
	}
	if file.Status == string(domain.FileStatusDeleting) {
		return fileaction.ErrFileNotFound
	}
	switch status {
	case domain.AttachmentReady:
		if file.Status != string(domain.FileStatusUploaded) {
			return fileaction.ErrFileNotFound
		}
		if _, err := tx.NewUpdate().Model(file).Set("status = ?", domain.FileStatusActive).Set("expires_at = NULL").Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
			return err
		}
	case domain.AttachmentUploading:
		if _, err := tx.NewUpdate().Model(file).Set("expires_at = now() + interval '24 hours'").Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
			return err
		}
	case domain.AttachmentCancelled:
		if _, err := tx.NewUpdate().Model(file).Set("status = ?", domain.FileStatusDeleting).Set("expires_at = now()").Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model((*servermodels.Message)(nil)).Set("deleted_at = now()").Set("updated_at = now()").Where("id = ?", row.MessageID).Exec(ctx); err != nil {
			return err
		}
		// 撤去尚未完成的末条附件时，摘要回到当前最后一条可见消息。
		if _, err := tx.NewRaw(`UPDATE conversations SET (last_message_id, last_message_at, last_message_source_order) =
 (SELECT latest.id, latest.originated_at, COALESCE(latest.source_order, 0) FROM (SELECT 1) AS anchor LEFT JOIN LATERAL
 (SELECT id, originated_at, source_order FROM messages WHERE organization_id = ? AND conversation_id = ? AND deleted_at IS NULL ORDER BY originated_at DESC, source_order DESC, id DESC LIMIT 1) latest ON true), updated_at = now()
 WHERE organization_id = ? AND id = ? AND last_message_id = ?`, identity.Organization.ID, row.ConversationID, identity.Organization.ID, row.ConversationID, row.MessageID).Exec(ctx); err != nil {
			return err
		}
	}
	_, err = tx.NewRaw(`UPDATE message_attachments SET upload_status = ?, upload_expires_at = CASE WHEN ? = 'uploading' THEN now() + interval '2 minutes' END WHERE message_id = ?`, status, status, row.MessageID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("update attachment upload: %w", err)
	}
	return nil
}
