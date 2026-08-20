package domain

// MessageAuthor 定义消息发送方。
type MessageAuthor string

const (
	MessageAuthorVisitor MessageAuthor = "visitor"
	MessageAuthorAgent   MessageAuthor = "agent"
)
