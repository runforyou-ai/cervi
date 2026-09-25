//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"mime"
	"net/url"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// SendAttachmentMessage 保存已上传的内部会话附件，并返回消息及首发创建的单聊或 AI 聊天。
func (o *directOperations) SendAttachmentMessage(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input AttachmentMessageInput) (AttachmentMessageResult, error) {
	result, err := o.sendAttachmentMessage.Execute(ctx, identity, conversationaction.AttachmentMessageInput{
		ConversationID: input.ConversationID, TargetIdentityID: input.TargetIdentityID, AgentIdentityID: input.AgentIdentityID, ServedConversationID: input.ServedConversationID,
		ClientMessageID: input.ClientMessageID, FileID: input.FileID, Body: input.Body, ImageWidth: input.ImageWidth, ImageHeight: input.ImageHeight,
	})
	if errors.Is(err, fileaction.ErrFileNotFound) {
		return AttachmentMessageResult{}, NotFoundError(meta, cervii18n.ErrorFileNotFound)
	}
	if err != nil {
		return AttachmentMessageResult{}, individualConversationError(ctx, meta, err, identity.Organization.ID, input.ConversationID, "send_attachment")
	}
	output := AttachmentMessageResult{ConversationID: result.ConversationID, Message: o.conversationMessageWithAvatar(ctx, identity, result.Message)}
	if result.Conversation != nil {
		urls, err := o.conversationAvatarURLs(ctx, identity, nil, result.Conversation.PeerAvatarFileID)
		if err != nil {
			slog.Warn("读取附件首发单聊头像失败", "conversation_id", result.ConversationID, "error", err)
		}
		conversation := directInboxConversationFromSummary(*result.Conversation, urls)
		output.Conversation = &conversation
	}
	if result.AgentConversation != nil {
		urls, err := o.conversationAvatarURLs(ctx, identity, nil, result.AgentConversation.Agent.AgentAvatarFileID)
		if err != nil {
			slog.Warn("读取附件首发 AI 聊天头像失败", "conversation_id", result.ConversationID, "error", err)
		}
		conversation := inboxConversationFromAction(*result.AgentConversation, urls)
		output.Conversation = &conversation
	}
	slog.Info("聊天附件消息已保存", "organization_id", identity.Organization.ID, "conversation_id", result.ConversationID, "message_id", result.Message.ID, "file_id", input.FileID)
	return output, nil
}

// GetAttachmentDownload 按文件实际存储位置创建带原始文件名的下载请求。
func (o *directOperations) GetAttachmentDownload(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID, messageID string) (FileDownload, error) {
	record, err := o.listConversationMessages.GetAttachmentFile(ctx, identity, conversationID, messageID)
	if err != nil {
		return FileDownload{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileNotFound)
	}
	if record.StorageBackend == string(domain.FileStorageBackendLocal) {
		contentURL, err := fileContentURL(domain.FileStorageBackendLocal, record.StorageKey, "")
		if err != nil {
			return FileDownload{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileNotFound)
		}
		return FileDownload{URL: contentURL + "?download=" + url.QueryEscape(record.OriginalName), PreviewURL: contentURL + "?inline=1"}, nil
	}
	request, err := serverfilecontent.PresignDownload(ctx, o.s3, record.StorageKey, mime.FormatMediaType("attachment", map[string]string{"filename": record.OriginalName}))
	if err != nil {
		return FileDownload{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileNotFound)
	}
	preview, err := serverfilecontent.PresignDownload(ctx, o.s3, record.StorageKey, "inline")
	if err != nil {
		return FileDownload{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorFileNotFound)
	}
	return FileDownload{URL: request.URL, PreviewURL: preview.URL}, nil
}
