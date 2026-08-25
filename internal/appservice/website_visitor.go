package appservice

import (
	"context"
	"time"
)

// WebsiteVisitorMeta 携带网站访客请求的本地化信息。
type WebsiteVisitorMeta struct {
	Locale Locale
}

// WebsiteVisitorServiceSession 定义客户线程最新客服处理状态。
type WebsiteVisitorServiceSession struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// WebsiteVisitorConversation 定义网站访客会话摘要。
type WebsiteVisitorConversation struct {
	ID             string                       `json:"id"`
	Title          string                       `json:"title"`
	Preview        string                       `json:"preview"`
	LastMessageAt  time.Time                    `json:"lastMessageAt"`
	ServiceSession WebsiteVisitorServiceSession `json:"serviceSession"`
}

// WebsiteVisitorMessenger 定义网站 Messenger 初始化结果。
type WebsiteVisitorMessenger struct {
	VisitorToken  string                       `json:"visitorToken"`
	Conversations []WebsiteVisitorConversation `json:"conversations"`
}

// WebsiteVisitorTextMessageInput 定义网站访客文本发送参数。
type WebsiteVisitorTextMessageInput struct {
	ClientMessageID string  `json:"clientMessageId"`
	ConversationID  *string `json:"conversationId"`
	Body            string  `json:"body"`
}

// WebsiteVisitorMessage 定义网站访客可见消息。
type WebsiteVisitorMessage struct {
	ID           string    `json:"id"`
	Author       string    `json:"author"`
	Body         string    `json:"body"`
	OriginatedAt time.Time `json:"originatedAt"`
	CreatedAt    time.Time `json:"createdAt"`
}

// WebsiteVisitorTextMessageResult 定义网站访客文本写入结果。
type WebsiteVisitorTextMessageResult struct {
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
	SendTextMessage(context.Context, WebsiteVisitorMeta, string, string, WebsiteVisitorTextMessageInput) (WebsiteVisitorTextMessageResult, error)
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
	conversations, err := s.backend.ListConversations(ctx, meta, channelID, externalID)
	if err != nil {
		return WebsiteVisitorMessenger{}, err
	}
	return WebsiteVisitorMessenger{VisitorToken: visitorToken, Conversations: conversations}, nil
}

// SendTextMessage 持久化网站访客文本消息。
func (s *WebsiteVisitorService) SendTextMessage(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID string, input WebsiteVisitorTextMessageInput) (WebsiteVisitorTextMessageResult, error) {
	return s.backend.SendTextMessage(ctx, meta, channelID, externalID, input)
}

// ListMessages 返回网站访客指定客户线程的消息历史。
func (s *WebsiteVisitorService) ListMessages(ctx context.Context, meta WebsiteVisitorMeta, channelID, externalID, conversationID string, input WebsiteVisitorMessageHistoryInput) (WebsiteVisitorMessageHistory, error) {
	return s.backend.ListMessages(ctx, meta, channelID, externalID, conversationID, input)
}
