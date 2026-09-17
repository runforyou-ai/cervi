package domain

// MessageAuthor 定义消息发送方。
type MessageAuthor string

const (
	MessageAuthorVisitor MessageAuthor = "visitor"
	MessageAuthorAgent   MessageAuthor = "agent"
)

// MessageVisibility 定义消息在客户会话中的可见范围。
type MessageVisibility string

const (
	MessageVisibilityCustomerVisible MessageVisibility = "customer_visible"
	MessageVisibilityInternalOnly    MessageVisibility = "internal_only"
)
