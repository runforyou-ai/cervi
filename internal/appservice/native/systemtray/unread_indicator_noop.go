//go:build !server

package systemtray

// noopUnreadIndicator 为暂未接入未读提示的平台保留统一能力占位。
type noopUnreadIndicator struct{}

// SetUnreadCount 接收未读消息数并保持当前平台界面不变。
func (*noopUnreadIndicator) SetUnreadCount(_ int) error {
	return nil
}
