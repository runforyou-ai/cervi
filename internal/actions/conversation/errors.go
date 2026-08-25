//go:build server

package conversation

import "errors"

var (
	// ErrChannelNotFound 表示网站渠道不存在或不可用。
	ErrChannelNotFound = errors.New("website channel not found")
	// ErrConversationNotFound 表示客户会话不属于当前渠道身份。
	ErrConversationNotFound = errors.New("customer conversation not found")
	// ErrDataInvariant 表示聊天持久关系不完整或互相矛盾。
	ErrDataInvariant = errors.New("conversation data invariant violated")
)

// ConflictError 表示语言无关的消息写入冲突。
type ConflictError struct {
	Reason string
}

// Error 返回稳定冲突原因。
func (e *ConflictError) Error() string { return "conversation conflict: " + e.Reason }

// ValidationError 表示公开访客输入不合法。
type ValidationError struct {
	Fields map[string]ValidationCode
}

// Error 返回输入校验失败。
func (e *ValidationError) Error() string { return "conversation input validation failed" }
