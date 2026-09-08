//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"mime"
	"net/url"
	"strings"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
)

// SendAttachmentMessage 保存内部会话附件并返回消息及首发会话。
func (b *DirectBackend) SendAttachmentMessage(ctx context.Context, meta RequestMeta, input AttachmentMessageInput) (AttachmentMessageResult, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return AttachmentMessageResult{}, err
	}
	result, err := b.sendAttachmentMessage.Execute(ctx, identity, conversationaction.AttachmentMessageInput{
		ConversationID: input.ConversationID, TargetIdentityID: input.TargetIdentityID, ClientMessageID: input.ClientMessageID, FileID: input.FileID,
	})
	if errors.Is(err, fileaction.ErrFileNotFound) {
		return AttachmentMessageResult{}, NotFoundError(meta, cervii18n.ErrorFileNotFound)
	}
	if err != nil {
		return AttachmentMessageResult{}, individualConversationError(ctx, meta, err, identity.Organization.ID, input.ConversationID, "send_attachment")
	}
	output := AttachmentMessageResult{ConversationID: result.ConversationID, Message: b.conversationMessageWithAvatar(ctx, identity, result.Message)}
	if result.Conversation != nil {
		urls, err := b.conversationAvatarURLs(ctx, identity, nil, result.Conversation.PeerAvatarFileID)
		if err != nil {
			slog.Warn("读取附件首发单聊头像失败", "error", err)
		}
		conversation := directInboxConversationFromSummary(*result.Conversation, urls)
		output.Conversation = &conversation
	}
	slog.Info("聊天附件消息已保存", "organization_id", identity.Organization.ID, "conversation_id", result.ConversationID, "message_id", result.Message.ID, "file_id", input.FileID)
	return output, nil
}

// GetAttachmentDownload 按文件实际存储位置创建带原始文件名的下载请求。
func (b *DirectBackend) GetAttachmentDownload(ctx context.Context, meta RequestMeta, conversationID, messageID string) (FileDownload, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return FileDownload{}, err
	}
	record, err := b.listConversationMessages.GetAttachmentFile(ctx, identity, conversationID, messageID)
	if err != nil {
		return FileDownload{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorFileNotFound)
	}
	if record.StorageBackend == string(domain.FileStorageBackendLocal) {
		contentURL, err := fileContentURL(domain.FileStorageBackendLocal, record.StorageKey, "")
		if err != nil {
			return FileDownload{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorFileNotFound)
		}
		return FileDownload{URL: contentURL + "?download=" + url.QueryEscape(record.OriginalName), PreviewURL: contentURL + "?inline=1"}, nil
	}
	setting, err := b.getS3Setting.ExecuteForOrganization(ctx, record.OrganizationID)
	if err != nil {
		return FileDownload{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorFileNotFound)
	}
	request, err := serverfilecontent.PresignDownload(ctx, s3FileConfig(setting), record.StorageKey, mime.FormatMediaType("attachment", map[string]string{"filename": record.OriginalName}))
	if err != nil {
		return FileDownload{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorFileNotFound)
	}
	preview, err := serverfilecontent.PresignDownload(ctx, s3FileConfig(setting), record.StorageKey, "inline")
	if err != nil {
		return FileDownload{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorFileNotFound)
	}
	return FileDownload{URL: request.URL, PreviewURL: preview.URL}, nil
}

// SendAttachmentBatch 保存单聊中的有序附件消息和说明文字。
func (b *DirectBackend) SendAttachmentBatch(ctx context.Context, meta RequestMeta, input AttachmentBatchInput) (AttachmentBatchResult, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return AttachmentBatchResult{}, err
	}
	items := make([]conversationaction.AttachmentMessageInput, 0, len(input.Attachments))
	for _, item := range input.Attachments {
		items = append(items, conversationaction.AttachmentMessageInput{FileID: item.FileID, ClientMessageID: item.ClientMessageID, ImageWidth: item.ImageWidth, ImageHeight: item.ImageHeight})
	}
	result, err := b.sendAttachmentMessage.ExecuteBatch(ctx, identity, conversationaction.AttachmentBatchInput{BatchID: input.BatchID, ConversationID: input.ConversationID, TargetIdentityID: input.TargetIdentityID, Body: input.Body, Attachments: items})
	if errors.Is(err, fileaction.ErrFileNotFound) {
		return AttachmentBatchResult{}, NotFoundError(meta, cervii18n.ErrorFileNotFound)
	}
	if err != nil {
		return AttachmentBatchResult{}, individualConversationError(ctx, meta, err, identity.Organization.ID, input.ConversationID, "send_attachments")
	}
	output := AttachmentBatchResult{ConversationID: result.ConversationID, Messages: make([]ConversationMessage, 0, len(result.Messages))}
	for _, message := range result.Messages {
		output.Messages = append(output.Messages, b.conversationMessageWithAvatar(ctx, identity, message))
	}
	if result.Conversation != nil {
		urls, err := b.conversationAvatarURLs(ctx, identity, nil, result.Conversation.PeerAvatarFileID)
		if err != nil {
			slog.Warn("读取附件单聊头像失败", "error", err)
		}
		conversation := directInboxConversationFromSummary(*result.Conversation, urls)
		output.Conversation = &conversation
	}
	slog.Info("单聊附件批次已保存", "conversation_id", result.ConversationID, "batch_id", input.BatchID, "message_count", len(result.Messages))
	return output, nil
}

// UpdateAttachmentUploads 更新当前发送者的附件内容状态。
func (b *DirectBackend) UpdateAttachmentUploads(ctx context.Context, meta RequestMeta, input AttachmentUploadUpdate) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.sendAttachmentMessage.UpdateUploads(ctx, identity, input.FileIDs, domain.AttachmentUploadStatus(input.Status)); err != nil {
		return b.fileOperationError(ctx, meta, err, cervii18n.ErrorFileUploadCompleteFailed)
	}
	if input.Status != AttachmentUploading {
		slog.Info("附件上传状态已更新", "file_count", len(input.FileIDs), "status", input.Status)
	}
	return nil
}

// ListAttachmentStates 刷新双方消息窗口里已有附件的内容状态。
func (b *DirectBackend) ListAttachmentStates(ctx context.Context, meta RequestMeta, conversationID string, input AttachmentStateListInput) (AttachmentStateList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return AttachmentStateList{}, err
	}
	states, err := b.listConversationMessages.AttachmentStates(ctx, identity, conversationID, strings.Split(input.MessageIDs, ","))
	if err != nil {
		return AttachmentStateList{}, individualConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "attachment_states")
	}
	output := AttachmentStateList{States: make([]AttachmentMessageState, 0, len(states))}
	for _, state := range states {
		mapped := conversationMessageFromAction(conversationaction.ConversationMessage{Attachment: &state.Attachment}, nil)
		output.States = append(output.States, AttachmentMessageState{MessageID: state.MessageID, Attachment: *mapped.Attachment, Deleted: state.Deleted})
	}
	return output, nil
}
