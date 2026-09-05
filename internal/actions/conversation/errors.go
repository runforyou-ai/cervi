//go:build server

package conversation

import (
	"errors"

	"github.com/runforyou-ai/cervi/internal/common"
)

var (
	// ErrMessageUnavailable 表示目标消息不存在或已经删除。
	ErrMessageUnavailable = errors.New("conversation message unavailable")
	// ErrMentionTargetInvalid 表示目标不是当前用户的提及。
	ErrMentionTargetInvalid = errors.New("conversation mention target invalid")
	// ErrMentionProgressChanged 表示目标之前仍有未确认的有效提及。
	ErrMentionProgressChanged = errors.New("conversation mention progress changed")
	// ErrChannelNotFound 表示网站渠道不存在或不可用。
	ErrChannelNotFound = errors.New("website channel not found")
	// ErrConversationNotFound 表示会话不存在或当前身份无权访问。
	ErrConversationNotFound = errors.New("conversation not found")
	// ErrDirectTargetNotFound 表示内部单聊目标不存在或不可用。
	ErrDirectTargetNotFound = errors.New("direct conversation target not found")
	// ErrGroupMemberNotFound 表示待加入群聊的成员不存在或不可用。
	ErrGroupMemberNotFound = errors.New("group conversation member not found")
	// ErrGroupImageFileNotFound 表示群聊图片文件不可关联。
	ErrGroupImageFileNotFound = errors.New("group conversation image file not found")
	// ErrGroupOwnerRequired 表示当前成员不是群主。
	ErrGroupOwnerRequired = errors.New("group conversation owner required")
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
