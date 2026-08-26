package appservice

import "github.com/runforyou-ai/cervi/internal/domain"

// MessageAuthor 表示消息发送方。
type MessageAuthor string

const (
	MessageAuthorVisitor MessageAuthor = MessageAuthor(domain.MessageAuthorVisitor)
	MessageAuthorAgent   MessageAuthor = MessageAuthor(domain.MessageAuthorAgent)
)

// Conversation 定义收件箱中的会话。
type Conversation struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Initials string    `json:"initials"`
	Channel  string    `json:"channel"`
	Preview  string    `json:"preview"`
	Time     string    `json:"time"`
	Status   string    `json:"status"`
	Unread   int       `json:"unread,omitempty"`
	Online   bool      `json:"online,omitempty"`
	Messages []Message `json:"messages"`
}

// Message 定义收件箱会话中的消息。
type Message struct {
	ID     string        `json:"id"`
	Author MessageAuthor `json:"author"`
	Text   string        `json:"text"`
	Time   string        `json:"time"`
}

// Inbox 定义统一收件箱结果。
type Inbox struct {
	Organization  Organization   `json:"organization"`
	User          CurrentUser    `json:"user"`
	Conversations []Conversation `json:"conversations"`
}
