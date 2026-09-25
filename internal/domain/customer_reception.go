package domain

// CustomerReceptionReply 定义访客端展示的回复预期：immediate 为立即回复，soon 为尽快回复，scheduled 为下个工作时段回复，none 为不展示。
type CustomerReceptionReply string

const (
	CustomerReceptionReplyImmediate CustomerReceptionReply = "immediate"
	CustomerReceptionReplySoon      CustomerReceptionReply = "soon"
	CustomerReceptionReplyScheduled CustomerReceptionReply = "scheduled"
	CustomerReceptionReplyNone      CustomerReceptionReply = "none"
)
