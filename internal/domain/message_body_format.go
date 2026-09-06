package domain

// MessageBodyFormat 定义消息正文的解释方式。
type MessageBodyFormat string

const (
	MessageBodyFormatPlain    MessageBodyFormat = "plain"
	MessageBodyFormatMarkdown MessageBodyFormat = "markdown"
)
