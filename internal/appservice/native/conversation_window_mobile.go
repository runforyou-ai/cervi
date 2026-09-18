//go:build !server && (android || ios)

package native

import "github.com/runforyou-ai/cervi/internal/appservice"

// NewConversationWindowOpener 禁用移动端会话独立窗口能力。
func NewConversationWindowOpener() appservice.ConversationWindowOpener {
	return nil
}
