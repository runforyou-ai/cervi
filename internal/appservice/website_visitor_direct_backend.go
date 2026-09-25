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
	"github.com/runforyou-ai/cervi/internal/actions/customernotify"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

var _ WebsiteVisitorBackend = (*WebsiteVisitorDirectBackend)(nil)

const (
	// WebsiteVisitorTokenHeader 是访客直传本地对象和调用公开接口使用的令牌请求头。
	WebsiteVisitorTokenHeader = "X-Cervi-Visitor-Token"
	// WebsiteCustomerTokenHeader 是网站登录用户直传本地对象和调用公开接口携带签名身份的请求头。
	WebsiteCustomerTokenHeader = "X-Cervi-Customer-Token"
	// WebsiteCustomerIdentityInvalidReason 是签名身份失效错误的稳定原因码。
	WebsiteCustomerIdentityInvalidReason = "customer_identity_invalid"
)

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
	verifyCustomer    *conversationaction.VerifyWebsiteCustomerQuery
	createUpload      *conversationaction.CreateWebsiteVisitorUploadAction
	completeUpload    *conversationaction.CompleteWebsiteVisitorUploadAction
	getAttachment     *conversationaction.GetWebsiteVisitorAttachmentQuery
	reportTyping      *conversationaction.ReportWebsiteVisitorTypingAction
	rateSession       *conversationaction.RateWebsiteServiceSessionAction
	markRead          *conversationaction.MarkWebsiteConversationReadAction
	resumeVisitor     *conversationaction.ResumeWebsiteVisitorQuery
	localFiles        *serverfilecontent.LocalStore
	s3                serverfilecontent.S3Config
}

// NewWebsiteVisitorDirectBackend 创建匿名网站访客直接后端；emailSender 为空表示部署未配置邮件发送。
func NewWebsiteVisitorDirectBackend(db *bun.DB, agentScheduler conversationaction.CustomerAgentMessageScheduler, taskEnqueuer servertask.TxEnqueuer, localFiles *serverfilecontent.LocalStore, s3 serverfilecontent.S3Config, emailSender customernotify.Sender) *WebsiteVisitorDirectBackend {
	backend := &WebsiteVisitorDirectBackend{
		listConversations: conversationaction.NewListWebsiteConversationsQuery(db),
		sendMessage:       conversationaction.NewReceiveWebsiteCustomerMessageAction(db, agentScheduler, taskEnqueuer, emailSender),
		listMessages:      conversationaction.NewListWebsiteMessagesQuery(db),
		authorizeVisitor:  conversationaction.NewAuthorizeWebsiteVisitorQuery(db),
		verifyCustomer:    conversationaction.NewVerifyWebsiteCustomerQuery(db),
		completeUpload:    conversationaction.NewCompleteWebsiteVisitorUploadAction(db),
		getAttachment:     conversationaction.NewGetWebsiteVisitorAttachmentQuery(db),
		reportTyping:      conversationaction.NewReportWebsiteVisitorTypingAction(db),
		rateSession:       conversationaction.NewRateWebsiteServiceSessionAction(db, taskEnqueuer),
		markRead:          conversationaction.NewMarkWebsiteConversationReadAction(db),
		resumeVisitor:     conversationaction.NewResumeWebsiteVisitorQuery(db),
		localFiles:        localFiles,
		s3:                s3,
	}
	backend.createUpload = conversationaction.NewCreateWebsiteVisitorUploadAction(db, func(context.Context, string) (domain.FileStorageBackend, error) {
		if s3.Enabled {
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

// VerifyCustomer 按渠道所属企业的客户身份密钥校验签名身份。
func (b *WebsiteVisitorDirectBackend) VerifyCustomer(ctx context.Context, meta WebsiteVisitorMeta, channelID, token string) (WebsiteVisitorCustomer, error) {
	verified, err := b.verifyCustomer.Execute(ctx, channelID, token)
	if err != nil {
		return WebsiteVisitorCustomer{}, websiteVisitorError(ctx, meta, err, cervii18n.ErrorCustomerIdentityInvalid, "verify_customer", "channel_id", channelID)
	}
	return websiteVisitorCustomerFromAction(verified), nil
}

// VerifyOrganizationCustomer 按指定企业的客户身份密钥校验签名身份，供访客直传本地对象时认证。
func (b *WebsiteVisitorDirectBackend) VerifyOrganizationCustomer(ctx context.Context, organizationID, token string) (WebsiteVisitorCustomer, error) {
	verified, err := b.verifyCustomer.ExecuteForOrganization(ctx, organizationID, token)
	if err != nil {
		return WebsiteVisitorCustomer{}, err
	}
	return websiteVisitorCustomerFromAction(verified), nil
}

// websiteVisitorCustomerFromAction 转换验签通过的网站登录用户。
func websiteVisitorCustomerFromAction(value conversationaction.VerifiedWebsiteCustomer) WebsiteVisitorCustomer {
	return WebsiteVisitorCustomer{
		OrganizationID: value.OrganizationID, UserID: value.Customer.UserID, Name: value.Customer.Name,
		Email: value.Customer.Email, ExpiresAt: value.ExpiresAt,
	}
}

// websiteCustomerInput 返回访客元信息中已验证的登录用户，匿名访客返回空。
func websiteCustomerInput(meta WebsiteVisitorMeta) *conversationaction.WebsiteCustomer {
	if meta.Customer == nil {
		return nil
	}
	return &conversationaction.WebsiteCustomer{UserID: meta.Customer.UserID, Name: meta.Customer.Name, Email: meta.Customer.Email}
}

// SendTextMessage 持久化网站访客文本消息。
func (b *WebsiteVisitorDirectBackend) SendTextMessage(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID string, input WebsiteVisitorTextMessageInput) (WebsiteVisitorMessageResult, error) {
	result, err := b.sendMessage.Execute(ctx, conversationaction.WebsiteCustomerTextMessageInput{
		ChannelID: channelID, ExternalID: externalID, ConversationID: input.ConversationID,
		ClientMessageID: input.ClientMessageID, Body: input.Body, ReplyToMessageID: input.ReplyToMessageID,
		Customer: websiteCustomerInput(meta), VisitorContext: websiteVisitorContext(meta, input.Page),
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
		Customer: websiteCustomerInput(meta), VisitorContext: websiteVisitorContext(meta, input.Page),
	})
	if err != nil {
		return WebsiteVisitorMessageResult{}, websiteVisitorError(ctx, meta, err, cervii18n.ErrorMessageSendFailed, "send_attachment_message", "channel_id", channelID)
	}
	return b.sentMessageResult(ctx, meta, channelID, "send_attachment_message", result)
}

// sentMessageResult 转换访客消息写入结果并记录保存日志。
func (b *WebsiteVisitorDirectBackend) sentMessageResult(ctx context.Context, meta WebsiteVisitorMeta, channelID, operation string, result conversationaction.ReceiveWebsiteCustomerMessageResult) (WebsiteVisitorMessageResult, error) {
	linker := visitorAttachmentLinker{s3: b.s3}
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
		Customer: websiteCustomerInput(meta),
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
	linker := visitorAttachmentLinker{s3: b.s3}
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
		// 登录用户以签名身份直传，匿名访客以访客令牌直传。
		headers := map[string]string{WebsiteVisitorTokenHeader: meta.Token, "Content-Type": record.ContentType}
		if meta.Customer != nil {
			headers = map[string]string{WebsiteCustomerTokenHeader: meta.CustomerToken, "Content-Type": record.ContentType}
		}
		return WebsiteVisitorUploadRequest{Method: http.MethodPut, URL: contentURL, Headers: headers}, nil
	}
	signed, err := serverfilecontent.PresignPut(ctx, b.s3, record.StorageKey, record.ContentType)
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
	info, err := serverfilecontent.Stat(ctx, b.s3, record.StorageKey)
	if err != nil {
		return "", 0, err
	}
	return info.ETag, info.ByteSize, nil
}

// visitorAttachmentLinker 按部署级对象存储配置签发访客附件地址。
type visitorAttachmentLinker struct {
	s3 serverfilecontent.S3Config
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
	preview, err := serverfilecontent.PresignDownload(ctx, l.s3, storageKey, "inline")
	if err != nil {
		return WebsiteVisitorAttachmentLinks{}, err
	}
	download, err := serverfilecontent.PresignDownload(ctx, l.s3, storageKey, mime.FormatMediaType("attachment", map[string]string{"filename": fileName}))
	if err != nil {
		return WebsiteVisitorAttachmentLinks{}, err
	}
	return WebsiteVisitorAttachmentLinks{PreviewURL: preview.URL, DownloadURL: download.URL}, nil
}

// avatarURL 返回头像文件的稳定公开地址。
func (l *visitorAttachmentLinker) avatarURL(_ context.Context, location conversationaction.FileLocation) (string, error) {
	return fileContentURL(location.StorageBackend, location.StorageKey, l.s3.PublicBaseURL)
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
	result := WebsiteVisitorMessageHistory{Messages: make([]WebsiteVisitorMessage, 0, len(page.Messages)), SessionRatings: make([]WebsiteVisitorSessionRating, 0, len(page.SessionRatings))}
	for _, rating := range page.SessionRatings {
		result.SessionRatings = append(result.SessionRatings, WebsiteVisitorSessionRating{
			ServiceSessionID: rating.ServiceSessionID, EndMessageID: rating.EndMessageID, WebsiteVisitorRating: websiteVisitorRatingFromAction(rating.VisitorRating),
		})
	}
	linker := visitorAttachmentLinker{s3: b.s3}
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

// RateServiceSession 保存网站访客对已关闭客服处理周期的评价。
func (b *WebsiteVisitorDirectBackend) RateServiceSession(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID, conversationID, serviceSessionID string, input WebsiteVisitorRatingInput) (WebsiteVisitorRating, error) {
	rating, err := b.rateSession.Execute(ctx, conversationaction.WebsiteServiceSessionRatingInput{
		ChannelID: channelID, ExternalID: externalID, ConversationID: conversationID, ServiceSessionID: serviceSessionID,
		Resolved: input.Resolved, Comment: input.Comment,
	})
	if err != nil {
		return WebsiteVisitorRating{}, websiteVisitorError(ctx, meta, err, cervii18n.ErrorServiceSessionRateFailed, "rate_service_session", "channel_id", channelID, "service_session_id", serviceSessionID)
	}
	slog.Info("网站访客已评价客服处理周期", "channel_id", channelID, "conversation_id", conversationID, "service_session_id", serviceSessionID, "resolved", input.Resolved)
	return websiteVisitorRatingFromAction(rating), nil
}

// MarkConversationRead 记录网站访客在客户线程中已读到的位置。
func (b *WebsiteVisitorDirectBackend) MarkConversationRead(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID, conversationID string, input WebsiteVisitorReadInput) error {
	messageSeq, err := strconv.ParseInt(input.MessageSeq, 10, 64)
	if err != nil {
		return InvalidError(RequestMeta{Locale: meta.Locale}, cervii18n.ErrorValidationFailed, nil)
	}
	if err := b.markRead.Execute(ctx, channelID, externalID, conversationID, messageSeq); err != nil {
		return websiteVisitorError(ctx, meta, err, cervii18n.ErrorServerUnavailable, "mark_conversation_read", "channel_id", channelID, "conversation_id", conversationID)
	}
	return nil
}

// ResumeVisitor 用邮件中的回访令牌恢复匿名访客身份。
func (b *WebsiteVisitorDirectBackend) ResumeVisitor(ctx context.Context, meta WebsiteVisitorMeta, channelID string, input WebsiteVisitorResumeInput) (WebsiteVisitorResume, error) {
	resumed, err := b.resumeVisitor.Execute(ctx, channelID, input.Token)
	if err != nil {
		return WebsiteVisitorResume{}, websiteVisitorError(ctx, meta, err, cervii18n.ErrorWebsiteMessengerLoadFailed, "resume_visitor", "channel_id", channelID)
	}
	return WebsiteVisitorResume{VisitorToken: resumed.VisitorToken, ConversationID: resumed.ConversationID}, nil
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
	if errors.Is(err, conversationaction.ErrCustomerIdentityInvalid) {
		return InvalidError(requestMeta, cervii18n.ErrorCustomerIdentityInvalid, nil).WithReason(WebsiteCustomerIdentityInvalidReason).WithStatus(http.StatusUnauthorized)
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
		case conversationaction.ConflictReasonServiceSessionNotRateable:
			return ConflictError(requestMeta, cervii18n.ErrorServiceSessionNotRateable, conflict.Reason)
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
	conversationaction.ValidationRatingCommentTooLong:    cervii18n.FieldRatingCommentTooLong,
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
	var event *WebsiteVisitorEvent
	if value.Event != nil {
		event = &WebsiteVisitorEvent{Type: string(value.Event.Type), ServiceSessionID: value.Event.ServiceSessionID, MemberName: value.Event.MemberName, Email: value.Event.Email}
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
		Event:      event,
		ID:         value.ID, Author: string(value.Author), Body: value.Body, SenderIdentityType: (*OrganizationIdentityType)(value.SenderIdentityType),
		MessageSeq: strconv.FormatInt(value.MessageSeq, 10), OriginatedAt: value.OriginatedAt, CreatedAt: value.CreatedAt,
	}, nil
}

// websiteVisitorRatingFromAction 转换访客评价状态。
func websiteVisitorRatingFromAction(value conversationaction.VisitorRating) WebsiteVisitorRating {
	return WebsiteVisitorRating{Rateable: value.Rateable, Resolved: value.Resolved, Comment: value.Comment}
}
