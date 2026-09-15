package domain

// CustomerReplyMode 定义 AI 写回复的生成方式。
type CustomerReplyMode string

const (
	// CustomerReplyModeReply 根据会话上下文撰写回复。
	CustomerReplyModeReply CustomerReplyMode = "reply"
	// CustomerReplyModeRewrite 保留客服草稿原意并改写表达。
	CustomerReplyModeRewrite CustomerReplyMode = "rewrite"
)

// CustomerReplyTone 定义 AI 写回复的语气。
type CustomerReplyTone string

const (
	CustomerReplyToneKeep         CustomerReplyTone = "keep"
	CustomerReplyToneProfessional CustomerReplyTone = "professional"
	CustomerReplyToneFriendly     CustomerReplyTone = "friendly"
	CustomerReplyToneConcise      CustomerReplyTone = "concise"
)
