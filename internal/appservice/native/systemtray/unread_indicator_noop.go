//go:build !server

package systemtray

import "github.com/runforyou-ai/cervi/internal/appservice"

// noopUnreadIndicator 忽略原生未读状态。
type noopUnreadIndicator struct{}

// SetUnreadState 忽略未读状态。
func (*noopUnreadIndicator) SetUnreadState(_ appservice.UnreadIndicatorState) error {
	return nil
}
