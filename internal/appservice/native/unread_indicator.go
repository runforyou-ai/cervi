//go:build !server

package native

// NoopUnreadIndicator 为暂未接入原生角标的平台保留统一能力占位。
type NoopUnreadIndicator struct{}

// NewNoopUnreadIndicator 创建不改变平台界面的未读指示器。
func NewNoopUnreadIndicator() *NoopUnreadIndicator {
	return &NoopUnreadIndicator{}
}

// SetUnreadCount 接收未读消息数并保持当前平台界面不变。
func (*NoopUnreadIndicator) SetUnreadCount(_ int) error {
	return nil
}
