package appservice

import (
	"context"
	"time"
)

// WebsiteVisitorMeta 携带网站访客请求的本地化信息和访客令牌。
type WebsiteVisitorMeta struct {
	Locale Locale
	Token  string
}

// WebsiteVisitorServiceSession 定义客户线程最新客服处理状态。
type WebsiteVisitorServiceSession struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// WebsiteVisitorConversation 定义网站访客会话摘要。
type WebsiteVisitorConversation struct {
	LastMessageSeq            string                       `json:"lastMessageSeq"`
	ID                        string                       `json:"id"`
	Title                     string                       `json:"title"`
	Preview                   string                       `json:"preview"`
	PreviewSenderIdentityType *OrganizationIdentityType    `json:"previewSenderIdentityType"`
	LastMessageAt             time.Time                    `json:"lastMessageAt"`
	ServiceSession            WebsiteVisitorServiceSession `json:"serviceSession"`
}

// WebsiteVisitorDirectory 定义网站访客当前渠道身份下的客户线程目录。
type WebsiteVisitorDirectory struct {
	Conversations []WebsiteVisitorConversation `json:"conversations"`
}

// WebsiteVisitorMessenger 定义网站 Messenger 初始化结果。
type WebsiteVisitorMessenger struct {
	VisitorToken  string                       `json:"visitorToken"`
	Conversations []WebsiteVisitorConversation `json:"conversations"`
}

// WebsiteVisitorTextMessageInput 定义网站访客文本发送参数。
type WebsiteVisitorTextMessageInput struct {
	ReplyToMessageID string  `json:"replyToMessageId"`
	ClientMessageID  string  `json:"clientMessageId"`
	ConversationID   *string `json:"conversationId"`
	Body             string  `json:"body"`
}

// WebsiteVisitorUploadInput 定义网站访客附件上传的文件元数据。
type WebsiteVisitorUploadInput struct {
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	ByteSize    int64  `json:"byteSize"`
}

// WebsiteVisitorUploadRequest 定义访客直传附件内容的请求。
type WebsiteVisitorUploadRequest struct {
	Headers map[string]string `json:"headers"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
}

// WebsiteVisitorUpload 定义访客附件上传结果。
type WebsiteVisitorUpload struct {
	FileID  string                      `json:"fileId"`
	Request WebsiteVisitorUploadRequest `json:"request"`
}

// WebsiteVisitorAttachmentMessageInput 定义网站访客附件发送参数。
type WebsiteVisitorAttachmentMessageInput struct {
	ReplyToMessageID string  `json:"replyToMessageId"`
	ClientMessageID  string  `json:"clientMessageId"`
	ConversationID   *string `json:"conversationId"`
	FileID           string  `json:"fileId"`
	Body             string  `json:"body"`
	ImageWidth       int     `json:"imageWidth"`
	ImageHeight      int     `json:"imageHeight"`
}

// WebsiteVisitorAttachmentLinks 定义访客附件的预览与下载地址，内容未就绪时两者为空。
type WebsiteVisitorAttachmentLinks struct {
	PreviewURL  string `json:"previewUrl"`
	DownloadURL string `json:"downloadUrl"`
}

// WebsiteVisitorAttachment 定义网站访客可见的消息附件。
type WebsiteVisitorAttachment struct {
	Name           string `json:"name"`
	ContentType    string `json:"contentType"`
	TransferStatus string `json:"transferStatus"`
	PreviewURL     string `json:"previewUrl"`
	DownloadURL    string `json:"downloadUrl"`
	ByteSize       int64  `json:"byteSize"`
	ImageWidth     int    `json:"imageWidth"`
	ImageHeight    int    `json:"imageHeight"`
}

// WebsiteVisitorMessageReference 定义网站访客可见的一层引用摘要。
type WebsiteVisitorMessageReference struct {
	SenderIdentityType *OrganizationIdentityType `json:"senderIdentityType"`
	ID                 string                    `json:"id"`
	Deleted            bool                      `json:"deleted"`
	Author             string                    `json:"author,omitempty"`
	Body               string                    `json:"body,omitempty"`
}

// WebsiteVisitorMessage 定义网站访客可见消息。
type WebsiteVisitorMessage struct {
	// ClientMessageID 仅向原发送身份返回。
	ClientMessageID    *string                         `json:"clientMessageId"`
	MessageSeq         string                          `json:"messageSeq"`
	ReplyTo            *WebsiteVisitorMessageReference `json:"replyTo"`
	Attachment         *WebsiteVisitorAttachment       `json:"attachment"`
	ID                 string                          `json:"id"`
	Author             string                          `json:"author"`
	Body               string                          `json:"body"`
	SenderIdentityType *OrganizationIdentityType       `json:"senderIdentityType"`
	OriginatedAt       time.Time                       `json:"originatedAt"`
	CreatedAt          time.Time                       `json:"createdAt"`
}

// WebsiteVisitorMessageResult 定义网站访客消息写入结果。
type WebsiteVisitorMessageResult struct {
	Conversation            WebsiteVisitorConversation `json:"conversation"`
	CreatedConversation     bool                       `json:"createdConversation"`
	OpenedNewServiceSession bool                       `json:"openedNewServiceSession"`
	Message                 WebsiteVisitorMessage      `json:"message"`
}

// WebsiteVisitorMessageHistoryInput 定义网站访客历史分页参数。
type WebsiteVisitorMessageHistoryInput struct {
	Before string
	After  string
}

// WebsiteVisitorMessageHistory 定义网站访客历史分页结果。
type WebsiteVisitorMessageHistory struct {
	Messages []WebsiteVisitorMessage `json:"messages"`
	Before   *string                 `json:"before"`
	After    *string                 `json:"after"`
}

// WebsiteVisitorBackend 定义匿名网站访客业务调用。
type WebsiteVisitorBackend interface {
	ListConversations(context.Context, WebsiteVisitorMeta, string, string) ([]WebsiteVisitorConversation, error)
	SendTextMessage(context.Context, WebsiteVisitorMeta, string, string, WebsiteVisitorTextMessageInput) (WebsiteVisitorMessageResult, error)
	SendAttachmentMessage(context.Context, WebsiteVisitorMeta, string, string, WebsiteVisitorAttachmentMessageInput) (WebsiteVisitorMessageResult, error)
	CreateAttachmentUpload(context.Context, WebsiteVisitorMeta, string, string, WebsiteVisitorUploadInput) (WebsiteVisitorUpload, error)
	CompleteAttachmentUpload(context.Context, WebsiteVisitorMeta, string, string, string) error
	GetMessageAttachment(context.Context, WebsiteVisitorMeta, string, string, string, string) (WebsiteVisitorAttachmentLinks, error)
	ListMessages(context.Context, WebsiteVisitorMeta, string, string, string, WebsiteVisitorMessageHistoryInput) (WebsiteVisitorMessageHistory, error)
}

// WebsiteVisitorService 转发匿名网站访客业务调用。
type WebsiteVisitorService struct {
	backend WebsiteVisitorBackend
}

// NewWebsiteVisitorService 创建网站访客应用服务。
func NewWebsiteVisitorService(backend WebsiteVisitorBackend) *WebsiteVisitorService {
	return &WebsiteVisitorService{backend: backend}
}

// InitializeMessenger 返回访客 Token 和当前渠道的会话列表。
func (s *WebsiteVisitorService) InitializeMessenger(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID, visitorToken string) (WebsiteVisitorMessenger, error) {
	directory, err := s.ListConversations(ctx, meta, channelID, externalID)
	if err != nil {
		return WebsiteVisitorMessenger{}, err
	}
	return WebsiteVisitorMessenger{VisitorToken: visitorToken, Conversations: directory.Conversations}, nil
}

// ListConversations 返回当前渠道身份的客户线程目录，供访客在初始化之后重新发现线程。
func (s *WebsiteVisitorService) ListConversations(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID string) (WebsiteVisitorDirectory, error) {
	conversations, err := s.backend.ListConversations(ctx, meta, channelID, externalID)
	if err != nil {
		return WebsiteVisitorDirectory{}, err
	}
	return WebsiteVisitorDirectory{Conversations: conversations}, nil
}

// SendTextMessage 持久化网站访客文本消息。
func (s *WebsiteVisitorService) SendTextMessage(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID string, input WebsiteVisitorTextMessageInput) (WebsiteVisitorMessageResult, error) {
	return s.backend.SendTextMessage(ctx, meta, channelID, externalID, input)
}

// SendAttachmentMessage 持久化网站访客附件消息并激活上传文件。
func (s *WebsiteVisitorService) SendAttachmentMessage(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID string, input WebsiteVisitorAttachmentMessageInput) (WebsiteVisitorMessageResult, error) {
	return s.backend.SendAttachmentMessage(ctx, meta, channelID, externalID, input)
}

// CreateAttachmentUpload 创建网站访客附件的上传请求。
func (s *WebsiteVisitorService) CreateAttachmentUpload(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID string, input WebsiteVisitorUploadInput) (WebsiteVisitorUpload, error) {
	return s.backend.CreateAttachmentUpload(ctx, meta, channelID, externalID, input)
}

// CompleteAttachmentUpload 核验网站访客上传的附件内容。
func (s *WebsiteVisitorService) CompleteAttachmentUpload(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID, fileID string) error {
	return s.backend.CompleteAttachmentUpload(ctx, meta, channelID, externalID, fileID)
}

// GetMessageAttachment 重新签发网站访客消息附件的预览与下载地址。
func (s *WebsiteVisitorService) GetMessageAttachment(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID, conversationID, messageID string) (WebsiteVisitorAttachmentLinks, error) {
	return s.backend.GetMessageAttachment(ctx, meta, channelID, externalID, conversationID, messageID)
}

// ListMessages 返回网站访客指定客户线程的消息历史。
func (s *WebsiteVisitorService) ListMessages(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID, conversationID string, input WebsiteVisitorMessageHistoryInput) (WebsiteVisitorMessageHistory, error) {
	return s.backend.ListMessages(ctx, meta, channelID, externalID, conversationID, input)
}
