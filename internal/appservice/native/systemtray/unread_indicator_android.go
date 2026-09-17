//go:build !server && android

package systemtray

import (
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// SetUnreadState 按未读总数维护 Android 通知，角标由未读通知本身呈现。
func (*Controller) SetUnreadState(state appservice.UnreadIndicatorState) error {
	if state.Count > 0 {
		return nil
	}
	// 未读归零和退出登录时撤回已投递的消息通知。
	application.Android.Notify(`{"action":"clear"}`)
	return nil
}
