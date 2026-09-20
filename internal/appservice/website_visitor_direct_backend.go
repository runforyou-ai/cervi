//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strconv"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

var _ WebsiteVisitorBackend = (*WebsiteVisitorDirectBackend)(nil)

// WebsiteVisitorTokenHeader 是访客直传本地对象和调用公开接口使用的令牌请求头。
const WebsiteVisitorTokenHeader = "X-Cervi-Visitor-Token"

// WebsiteVisitorAudience 是网站访客事件流的受众标识，渠道身份记录 ID 用于构造受众 Subject。
type WebsiteVisitorAudience struct {
	OrganizationID    string
	ChannelID         string
	ChannelIdentityID string
}

// WebsiteVisitorDirectBackend 在服务端进程内调用匿名访客 Action 和 Query。
type WebsiteVisitorDirectBackend struct {
	listConversations *conversationaction.ListWebsiteConversationsQuery
	sendMessage       *conversationaction.ReceiveWebsiteCustomerMessageAction
	listMessages      *conversationaction.ListWebsiteMessagesQuery
	authorizeVisitor  *conversationaction.AuthorizeWebsiteVisitorQuery
	createUpload      *conversationaction.CreateWebsiteVisitorUploadAction
	completeUpload    *conversationaction.CompleteWebsiteVisitorUploadAction
	getAttachment     *conversationaction.GetWebsiteVisitorAttachmentQuery
	reportTyping      *conversationaction.ReportWebsiteVisitorTypingAction
	getS3Setting      *settingaction.GetS3SettingQuery
	localFiles        *serverfilecontent.LocalStore
}

// NewWebsiteVisitorDirectBackend 创建匿名网站访客直接后端。
func NewWebsiteVisitorDirectBackend(db *bun.DB, agentScheduler conversationaction.CustomerAgentMessageScheduler, localFiles *serverfilecontent.LocalStore) *WebsiteVisitorDirectBackend {
	getS3Setting := settingaction.NewGetS3SettingQuery(db)
	backend := &WebsiteVisitorDirectBackend{
		listConversations: conversationaction.NewListWebsiteConversationsQuery(db),
		sendMessage:       conversationaction.NewReceiveWebsiteCustomerMessageAction(db, agentScheduler),
		listMessages:      conversationaction.NewListWebsiteMessagesQuery(db),
		authorizeVisitor:  conversationaction.NewAuthorizeWebsiteVisitorQuery(db),
		completeUpload:    conversationaction.NewCompleteWebsiteVisitorUploadAction(db),
		getAttachment:     conversationaction.NewGetWebsiteVisitorAttachmentQuery(db),
		reportTyping:      conversationaction.NewReportWebsiteVisitorTypingAction(db),
		getS3Setting:      getS3Setting,
		localFiles:        localFiles,
	}
	backend.createUpload = conversationaction.NewCreateWebsiteVisitorUploadAction(db, func(ctx context.Context, organizationID string) (domain.FileStorageBackend, error) {
		setting, err := getS3Setting.ExecuteForOrganization(ctx, organizationID)
		if err != nil {
			return "", err
		}
		if setting.Enabled {
			return domain.FileStorageBackendS3, nil
		}
		return domain.FileStorageBackendLocal, nil
	})
	return backend
}

// AuthenticateVisitor 解析访客事件流受众；渠道停用或尚未建立业务身份时返回与访客 HTTP 接口一致的错误。
func (b *WebsiteVisitorDirectBackend) AuthenticateVisitor(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID string) (WebsiteVisitorAudience, error) {
	audience, err := b.authorizeVisitor.Execute(ctx, channelID, externalID)
	if err != nil {
		return WebsiteVisitorAudience{}, websiteVisitorError(ctx, meta, err, cervii18n.ErrorWebsiteMessengerLoadFailed, "authenticate_visitor", "channel_id", channelID)
	}
	return WebsiteVisitorAudience{OrganizationID: audience.OrganizationID, ChannelID: audience.ChannelID, ChannelIdentityID: audience.ChannelIdentityID}, nil
}

// ListConversations 返回网站访客的客户会话列表。
func (b *WebsiteVisitorDirectBackend) ListConversations(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID string) ([]WebsiteVisitorConversation, error) {
	items, err := b.listConversations.Execute(ctx, channelID, externalID)
	if err != nil {
		return nil, websiteVisitorError(ctx, meta, err, cervii18n.ErrorWebsiteMessengerLoadFailed, "list_conversations", "channel_id", channelID)
	}
	result := make([]WebsiteVisitorConversation, 0, len(items))
	for _, item := range items {
		result = append(result, websiteVisitorConversationFromAction(item))
	}
	return result, nil
}

// SendTextMessage 持久化网站访客文本消息。
func (b *WebsiteVisitorDirectBackend) SendTextMessage(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID string, input WebsiteVisitorTextMessageInput) (WebsiteVisitorMessageResult, error) {
	result, err := b.sendMessage.Execute(ctx, conversationaction.WebsiteCustomerTextMessageInput{
		ChannelID: channelID, ExternalID: externalID, ConversationID: input.ConversationID,
		ClientMessageID: input.ClientMessageID, Body: input.Body, ReplyToMessageID: input.ReplyToMessageID,
	})
	if err != nil {
		return WebsiteVisitorMessageResult{}, websiteVisitorError(ctx, meta, err, cervii18n.ErrorMessageSendFailed, "send_text_message", "channel_id", channelID)
	}
	return b.sentMessageResult(ctx, meta, channelID, "send_text_message", result)
}

// SendAttachmentMessage 持久化网站访客附件消息并激活上传文件。
func (b *WebsiteVisitorDirectBackend) SendAttachmentMessage(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID string, input WebsiteVisitorAttachmentMessageInput) (WebsiteVisitorMessageResult, error) {
	result, err := b.sendMessage.ExecuteAttachment(ctx, conversationaction.WebsiteCustomerAttachmentMessageInput{
		ChannelID: channelID, ExternalID: externalID, ConversationID: input.ConversationID,
		ClientMessageID: input.ClientMessageID, FileID: input.FileID, Body: input.Body, ReplyToMessageID: input.ReplyToMessageID,
		ImageWidth: input.ImageWidth, ImageHeight: input.ImageHeight,
	})
	if err != nil {
		return WebsiteVisitorMessageResult{}, websiteVisitorError(ctx, meta, err, cervii18n.ErrorMessageSendFailed, "send_attachment_message", "channel_id", channelID)
	}
	return b.sentMessageResult(ctx, meta, channelID, "send_attachment_message", result)
}

// sentMessageResult 转换访客消息写入结果并记录保存日志。
func (b *WebsiteVisitorDirectBackend) sentMessageResult(ctx context.Context, meta WebsiteVisitorMeta, channelID, operation string, result conversationaction.ReceiveWebsiteCustomerMessageResult) (WebsiteVisitorMessageResult, error) {
	linker := visitorAttachmentLinker{backend: b, organizationID: result.OrganizationID}
	message, err := websiteVisitorMessageFromAction(ctx, &linker, result.Message)
	if err != nil {
		return WebsiteVisitorMessageResult{}, websiteVisitorError(ctx, meta, err, cervii18n.ErrorMessageSendFailed, operation, "channel_id", channelID)
	}
	slog.Info("网站访客消息已保存",
		"channel_id", channelID,
		"conversation_id", result.Conversation.ID,
		"service_session_id", result.Conversation.ServiceSessionID,
		"message_id", result.Message.ID,
		"created_conversation", result.CreatedConversation,
		"opened_new_service_session", result.OpenedNewServiceSession,
	)
	return WebsiteVisitorMessageResult{
		Conversation:            websiteVisitorConversationFromAction(result.Conversation),
		CreatedConversation:     result.CreatedConversation,
		OpenedNewServiceSession: result.OpenedNewServiceSession,
		Message:                 message,
	}, nil
}

// CreateAttachmentUpload 创建网站访客附件的上传请求。
func (b *WebsiteVisitorDirectBackend) CreateAttachmentUpload(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID string, input WebsiteVisitorUploadInput) (WebsiteVisitorUpload, error) {
	record, err := b.createUpload.Execute(ctx, conversationaction.WebsiteVisitorUploadInput{
		ChannelID: channelID, ExternalID: externalID,
		FileName: input.FileName, ContentType: input.ContentType, ByteSize: input.ByteSize,
	})
	if err != nil {
		return WebsiteVisitorUpload{}, websiteVisitorError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed, "create_attachment_upload", "channel_id", channelID)
	}
	request, err := b.visitorUploadRequest(ctx, meta, record)
	if err != nil {
		return WebsiteVisitorUpload{}, websiteVisitorError(ctx, meta, err, cervii18n.ErrorFileUploadCreateFailed, "create_attachment_upload", "channel_id", channelID)
	}
	return WebsiteVisitorUpload{FileID: record.ID, Request: request}, nil
}

// CompleteAttachmentUpload 核验网站访客上传的附件内容。
func (b *WebsiteVisitorDirectBackend) CompleteAttachmentUpload(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID, fileID string) error {
	record, err := b.completeUpload.Execute(ctx, channelID, externalID, fileID, b.statVisitorFile)
	if err != nil {
		return websiteVisitorError(ctx, meta, err, cervii18n.ErrorFileUploadCompleteFailed, "complete_attachment_upload", "channel_id", channelID, "file_id", fileID)
	}
	slog.Info("网站访客附件上传已完成", "organization_id", record.OrganizationID, "channel_id", channelID, "file_id", record.ID, "storage_backend", record.StorageBackend)
	return nil
}

// GetMessageAttachment 重新签发网站访客消息附件的预览与下载地址。
func (b *WebsiteVisitorDirectBackend) GetMessageAttachment(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID, conversationID, messageID string) (WebsiteVisitorAttachmentLinks, error) {
	record, err := b.getAttachment.Execute(ctx, channelID, externalID, conversationID, messageID)
	if err != nil {
		return WebsiteVisitorAttachmentLinks{}, websiteVisitorError(ctx, meta, err, cervii18n.ErrorFileNotFound, "get_message_attachment", "channel_id", channelID, "message_id", messageID)
	}
	linker := visitorAttachmentLinker{backend: b, organizationID: record.OrganizationID}
	links, err := linker.links(ctx, domain.FileStorageBackend(record.StorageBackend), record.StorageKey, record.OriginalName)
	if err != nil {
		return WebsiteVisitorAttachmentLinks{}, websiteVisitorError(ctx, meta, err, cervii18n.ErrorFileNotFound, "get_message_attachment", "channel_id", channelID, "message_id", messageID)
	}
	return links, nil
}

// visitorUploadRequest 返回访客直传的本地上传地址或 S3 预签名请求。
func (b *WebsiteVisitorDirectBackend) visitorUploadRequest(ctx context.Context, meta WebsiteVisitorMeta, record *servermodels.File) (WebsiteVisitorUploadRequest, error) {
	if record.StorageBackend == string(domain.FileStorageBackendLocal) {
		contentURL, err := fileContentURL(domain.FileStorageBackendLocal, record.StorageKey, "")
		if err != nil {
			return WebsiteVisitorUploadRequest{}, err
		}
		return WebsiteVisitorUploadRequest{
			Method: http.MethodPut, URL: contentURL,
			Headers: map[string]string{WebsiteVisitorTokenHeader: meta.Token, "Content-Type": record.ContentType},
		}, nil
	}
	setting, err := b.getS3Setting.ExecuteForOrganization(ctx, record.OrganizationID)
	if err != nil {
		return WebsiteVisitorUploadRequest{}, err
	}
	signed, err := serverfilecontent.PresignPut(ctx, s3FileConfig(setting), record.StorageKey, record.ContentType)
	if err != nil {
		return WebsiteVisitorUploadRequest{}, err
	}
	return WebsiteVisitorUploadRequest{Method: signed.Method, URL: signed.URL, Headers: signed.Headers}, nil
}

// statVisitorFile 按文件记录的存储类型核验访客上传的内容。
func (b *WebsiteVisitorDirectBackend) statVisitorFile(ctx context.Context, record *servermodels.File) (string, int64, error) {
	if record.StorageBackend == string(domain.FileStorageBackendLocal) {
		info, err := b.localFiles.Stat(ctx, record.StorageKey)
		if err != nil {
			return "", 0, err
		}
		return "", info.Size(), nil
	}
	setting, err := b.getS3Setting.ExecuteForOrganization(ctx, record.OrganizationID)
	if err != nil {
		return "", 0, err
	}
	info, err := serverfilecontent.Stat(ctx, s3FileConfig(setting), record.StorageKey)
	if err != nil {
		return "", 0, err
	}
	return info.ETag, info.ByteSize, nil
}

// visitorAttachmentLinker 按企业对象存储配置签发访客附件地址，S3 配置在首次需要时读取一次。
type visitorAttachmentLinker struct {
	backend        *WebsiteVisitorDirectBackend
	setting        *settingaction.S3Setting
	organizationID string
}

// links 返回附件的预览与下载地址。
func (l *visitorAttachmentLinker) links(ctx context.Context, backend domain.FileStorageBackend, storageKey, fileName string) (WebsiteVisitorAttachmentLinks, error) {
	if backend == domain.FileStorageBackendLocal {
		contentURL, err := fileContentURL(domain.FileStorageBackendLocal, storageKey, "")
		if err != nil {
			return WebsiteVisitorAttachmentLinks{}, err
		}
		return WebsiteVisitorAttachmentLinks{PreviewURL: contentURL + "?inline=1", DownloadURL: contentURL + "?download=" + url.QueryEscape(fileName)}, nil
	}
	setting, err := l.s3Setting(ctx)
	if err != nil {
		return WebsiteVisitorAttachmentLinks{}, err
	}
	preview, err := serverfilecontent.PresignDownload(ctx, s3FileConfig(setting), storageKey, "inline")
	if err != nil {
		return WebsiteVisitorAttachmentLinks{}, err
	}
	download, err := serverfilecontent.PresignDownload(ctx, s3FileConfig(setting), storageKey, mime.FormatMediaType("attachment", map[string]string{"filename": fileName}))
	if err != nil {
		return WebsiteVisitorAttachmentLinks{}, err
	}
	return WebsiteVisitorAttachmentLinks{PreviewURL: preview.URL, DownloadURL: download.URL}, nil
}

// avatarURL 返回头像文件的稳定公开地址。
func (l *visitorAttachmentLinker) avatarURL(ctx context.Context, location conversationaction.FileLocation) (string, error) {
	if location.StorageBackend == domain.FileStorageBackendLocal {
		return fileContentURL(location.StorageBackend, location.StorageKey, "")
	}
	setting, err := l.s3Setting(ctx)
	if err != nil {
		return "", err
	}
	return fileContentURL(location.StorageBackend, location.StorageKey, setting.PublicBaseURL)
}

// s3Setting 返回企业对象存储配置，同一次转换内只读取一次。
func (l *visitorAttachmentLinker) s3Setting(ctx context.Context) (settingaction.S3Setting, error) {
	if l.setting == nil {
		setting, err := l.backend.getS3Setting.ExecuteForOrganization(ctx, l.organizationID)
		if err != nil {
			return settingaction.S3Setting{}, err
		}
		l.setting = &setting
	}
	return *l.setting, nil
}

// ListMessages 返回网站访客指定客户线程的消息历史。
func (b *WebsiteVisitorDirectBackend) ListMessages(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID, conversationID string, input WebsiteVisitorMessageHistoryInput) (WebsiteVisitorMessageHistory, error) {
	actionInput := conversationaction.MessageHistoryInput{
		ChannelID: channelID, ExternalID: externalID, ConversationID: conversationID,
	}
	if input.Before != "" && input.After != "" {
		return WebsiteVisitorMessageHistory{}, websiteVisitorError(ctx, meta, &conversationaction.ValidationError{Fields: map[string]conversationaction.ValidationCode{"cursor": conversationaction.ValidationCursorInvalid}}, cervii18n.ErrorConversationMessageListFailed, "list_messages", "channel_id", channelID, "conversation_id", conversationID)
	}
	if input.Before != "" {
		point, valid := decodeConversationMessageCursor(input.Before, conversationID)
		if !valid {
			return WebsiteVisitorMessageHistory{}, websiteVisitorError(ctx, meta, &conversationaction.ValidationError{Fields: map[string]conversationaction.ValidationCode{"before": conversationaction.ValidationCursorInvalid}}, cervii18n.ErrorConversationMessageListFailed, "list_messages", "channel_id", channelID, "conversation_id", conversationID)
		}
		actionInput.Before = &point
	}
	if input.After != "" {
		point, valid := decodeConversationMessageCursor(input.After, conversationID)
		if !valid {
			return WebsiteVisitorMessageHistory{}, websiteVisitorError(ctx, meta, &conversationaction.ValidationError{Fields: map[string]conversationaction.ValidationCode{"after": conversationaction.ValidationCursorInvalid}}, cervii18n.ErrorConversationMessageListFailed, "list_messages", "channel_id", channelID, "conversation_id", conversationID)
		}
		actionInput.After = &point
	}
	page, err := b.listMessages.Execute(ctx, actionInput)
	if err != nil {
		return WebsiteVisitorMessageHistory{}, websiteVisitorError(ctx, meta, err, cervii18n.ErrorConversationMessageListFailed, "list_messages", "channel_id", channelID, "conversation_id", conversationID)
	}
	result := WebsiteVisitorMessageHistory{Messages: make([]WebsiteVisitorMessage, 0, len(page.Messages))}
	linker := visitorAttachmentLinker{backend: b, organizationID: page.OrganizationID}
	for _, message := range page.Messages {
		converted, err := websiteVisitorMessageFromAction(ctx, &linker, message)
		if err != nil {
			return WebsiteVisitorMessageHistory{}, websiteVisitorError(ctx, meta, err, cervii18n.ErrorConversationMessageListFailed, "list_messages", "channel_id", channelID, "conversation_id", conversationID)
		}
		result.Messages = append(result.Messages, converted)
	}
	if page.Before != nil {
		value := encodeConversationMessageCursor(conversationID, *page.Before)
		result.Before = &value
	}
	if page.After != nil {
		value := encodeConversationMessageCursor(conversationID, *page.After)
		result.After = &value
	}
	return result, nil
}

// ReportTyping 向企业客服发布网站访客在客户线程中的输入状态。
func (b *WebsiteVisitorDirectBackend) ReportTyping(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID, conversationID string, input WebsiteVisitorTypingInput) error {
	if err := b.reportTyping.Execute(ctx, channelID, externalID, conversationID, input.Active); err != nil {
		return websiteVisitorError(ctx, meta, err, cervii18n.ErrorServerUnavailable, "report_typing", "channel_id", channelID, "conversation_id", conversationID)
	}
	return nil
}

// websiteVisitorError 把语言无关访客错误映射为本地化应用错误。
func websiteVisitorError(ctx context.Context, meta WebsiteVisitorMeta, err error, failureKey cervii18n.Key, operation string, attributes ...any) error {
	requestMeta := RequestMeta{Locale: meta.Locale}
	if validation, ok := errors.AsType[*conversationaction.ValidationError](err); ok {
		return InvalidError(requestMeta, cervii18n.ErrorValidationFailed, translateValidationFields(validation.Fields, websiteVisitorValidationKeys))
	}
	if errors.Is(err, conversationaction.ErrChannelNotFound) {
		return NotFoundError(requestMeta, cervii18n.ErrorChannelNotFound)
	}
	if errors.Is(err, conversationaction.ErrConversationNotFound) {
		return NotFoundError(requestMeta, cervii18n.ErrorConversationNotFound)
	}
	if errors.Is(err, fileaction.ErrFileNotFound) {
		return NotFoundError(requestMeta, cervii18n.ErrorFileNotFound)
	}
	if conflict, ok := errors.AsType[*conversationaction.ConflictError](err); ok {
		switch conflict.Reason {
		case conversationaction.ConflictReasonReplyTargetInvalid:
			return ConflictError(requestMeta, cervii18n.ErrorReplyTargetInvalid, conflict.Reason)
		case conversationaction.ConflictReasonAttachmentTooLarge:
			return ConflictError(requestMeta, cervii18n.ErrorAttachmentTooLarge, conflict.Reason)
		}
		return ConflictError(requestMeta, cervii18n.ErrorMessageConflict, conflict.Reason)
	}
	// 请求上下文有效时记录操作失败告警。
	if ctx.Err() == nil {
		logAttributes := []any{"operation", operation}
		logAttributes = append(logAttributes, attributes...)
		logAttributes = append(logAttributes, "error", err)
		slog.Warn("网站访客操作失败", logAttributes...)
	}
	return FailedError(requestMeta, failureKey)
}

var websiteVisitorValidationKeys = map[conversationaction.ValidationCode]cervii18n.Key{
	conversationaction.ValidationReplyToMessageIDInvalid: cervii18n.FieldReplyToMessageIDInvalid,
	conversationaction.ValidationChannelIDInvalid:        cervii18n.FieldChannelIDInvalid,
	conversationaction.ValidationExternalIDInvalid:       cervii18n.FieldVisitorTokenInvalid,
	conversationaction.ValidationConversationIDInvalid:   cervii18n.FieldConversationIDInvalid,
	conversationaction.ValidationClientMessageIDInvalid:  cervii18n.FieldClientMessageIDInvalid,
	conversationaction.ValidationBodyRequired:            cervii18n.FieldMessageBodyRequired,
	conversationaction.ValidationBodyTooLong:             cervii18n.FieldMessageBodyTooLong,
	conversationaction.ValidationCursorInvalid:           cervii18n.FieldMessageCursorInvalid,
	conversationaction.ValidationFileIDInvalid:           cervii18n.ErrorFileNotFound,
	fileaction.ValidationFileNameRequired:                cervii18n.FieldFileNameRequired,
	fileaction.ValidationContentTypeInvalid:              cervii18n.FieldFileContentTypeInvalid,
	fileaction.ValidationByteSizeInvalid:                 cervii18n.FieldFileByteSizeInvalid,
	fileaction.ValidationPurposeInvalid:                  cervii18n.FieldFilePurposeInvalid,
}

// websiteVisitorConversationFromAction 转换访客会话摘要。
func websiteVisitorConversationFromAction(value conversationaction.ConversationSummary) WebsiteVisitorConversation {
	return WebsiteVisitorConversation{
		ID: value.ID, Title: value.Title, Preview: value.Preview, PreviewSenderIdentityType: (*OrganizationIdentityType)(value.PreviewSenderIdentityType), LastMessageSeq: strconv.FormatInt(value.LastMessageSeq, 10), LastMessageAt: value.LastMessageAt,
		ServiceSession: WebsiteVisitorServiceSession{ID: value.ServiceSessionID, Status: string(value.ServiceSessionStatus)},
	}
}

// websiteVisitorMessageFromAction 转换访客消息，已就绪的附件同时签发预览和下载地址。
func websiteVisitorMessageFromAction(ctx context.Context, linker *visitorAttachmentLinker, value conversationaction.Message) (WebsiteVisitorMessage, error) {
	var replyTo *WebsiteVisitorMessageReference
	if value.ReplyTo != nil {
		replyTo = &WebsiteVisitorMessageReference{
			ID: value.ReplyTo.ID, Deleted: value.ReplyTo.Deleted,
			Author: string(value.ReplyTo.Author), Body: value.ReplyTo.Body,
			SenderIdentityType: (*OrganizationIdentityType)(value.ReplyTo.SenderIdentityType),
		}
	}
	var attachment *WebsiteVisitorAttachment
	if value.Attachment != nil {
		attachment = &WebsiteVisitorAttachment{
			Name: value.Attachment.Name, ContentType: value.Attachment.ContentType, ByteSize: value.Attachment.ByteSize,
			ImageWidth: value.Attachment.ImageWidth, ImageHeight: value.Attachment.ImageHeight,
			TransferStatus: string(value.Attachment.TransferStatus),
		}
		// 内容尚未就绪的附件不返回地址。
		if value.Attachment.TransferStatus == domain.MessageAttachmentTransferReady {
			links, err := linker.links(ctx, value.Attachment.StorageBackend, value.Attachment.StorageKey, value.Attachment.Name)
			if err != nil {
				return WebsiteVisitorMessage{}, err
			}
			attachment.PreviewURL, attachment.DownloadURL = links.PreviewURL, links.DownloadURL
		}
	}
	senderAvatarURL := ""
	if value.SenderAvatar != nil {
		avatarURL, err := linker.avatarURL(ctx, *value.SenderAvatar)
		if err != nil {
			return WebsiteVisitorMessage{}, err
		}
		senderAvatarURL = avatarURL
	}
	return WebsiteVisitorMessage{
		ClientMessageID: value.ClientMessageID, SenderIdentityID: value.SenderIdentityID, SenderName: value.SenderDisplayName, SenderAvatarURL: senderAvatarURL,
		ReplyTo:    replyTo,
		Attachment: attachment,
		ID:         value.ID, Author: string(value.Author), Body: value.Body, SenderIdentityType: (*OrganizationIdentityType)(value.SenderIdentityType),
		MessageSeq: strconv.FormatInt(value.MessageSeq, 10), OriginatedAt: value.OriginatedAt, CreatedAt: value.CreatedAt,
	}, nil
}
