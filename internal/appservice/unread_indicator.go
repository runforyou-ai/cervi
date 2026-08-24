package appservice

import "strconv"

const maxDisplayedUnreadCount = 99

// UnreadIndicatorState 定义未读事实与当前是否需要吸引用户注意。
type UnreadIndicatorState struct {
	Count            int  `json:"count"`
	AttentionEnabled bool `json:"attentionEnabled"`
	AttentionPending bool `json:"attentionPending"`
}

// UnreadIndicator 将未读状态投射到当前平台的原生界面。
type UnreadIndicator interface {
	SetUnreadState(state UnreadIndicatorState) error
}

// FormatUnreadCount 将未读消息数格式化为适合原生角标展示的短文本。
func FormatUnreadCount(count int) string {
	if count <= 0 {
		return ""
	}
	if count > maxDisplayedUnreadCount {
		return "99+"
	}
	return strconv.Itoa(count)
}
