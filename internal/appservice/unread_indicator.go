package appservice

import "strconv"

const maxDisplayedUnreadCount = 99

// UnreadIndicator 将绝对未读消息数投射到当前平台的原生界面。
type UnreadIndicator interface {
	SetUnreadCount(count int) error
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
