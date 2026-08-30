//go:build server

package conversation

import (
	"errors"

	"github.com/runforyou-ai/cervi/internal/common"
)

var (
	// ErrChannelNotFound 表示网站渠道不存在或不可用。
	ErrChannelNotFound = errors.New("website channel not found")
	// ErrConversationNotFound 表示会话不存在或当前身份无权访问。
	ErrConversationNotFound = errors.New("conversation not found")
	// ErrDirectTargetNotFound 表示内部单聊目标不存在或不可用。
	ErrDirectTargetNotFound = errors.New("direct conversation target not found")
	// ErrDataInvariant 表示聊天持久关系不完整或互相矛盾。
	ErrDataInvariant = errors.New("conversation data invariant violated")
)

// ConflictError 表示语言无关的消息写入冲突。
type ConflictError struct {
	Reason string
}

// Error 返回稳定冲突原因。
func (e *ConflictError) Error() string { return "conversation conflict: " + e.Reason }

// ValidationError 表示会话业务输入不合法。
type ValidationError = common.FieldError
