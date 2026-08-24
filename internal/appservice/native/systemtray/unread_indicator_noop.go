//go:build !server

package systemtray

import "github.com/runforyou-ai/cervi/internal/appservice"

// noopUnreadIndicator 为暂未接入未读提示的平台保留统一能力占位。
type noopUnreadIndicator struct{}

// SetUnreadState 接收未读状态并保持当前平台界面不变。
func (*noopUnreadIndicator) SetUnreadState(_ appservice.UnreadIndicatorState) error {
	return nil
}
