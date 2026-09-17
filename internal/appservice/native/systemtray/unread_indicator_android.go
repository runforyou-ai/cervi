//go:build !server && android

package systemtray

import (
	"github.com/runforyou-ai/cervi/internal/appservice"
)

// SetUnreadState 保持 Android 角标由未读通知本身呈现。
func (*Controller) SetUnreadState(_ appservice.UnreadIndicatorState) error {
	return nil
}
