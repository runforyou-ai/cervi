package appservice

import "testing"

// TestFormatUnreadCount 验证未读消息数的跨平台展示格式。
func TestFormatUnreadCount(t *testing.T) {
	tests := []struct {
		name  string
		count int
		want  string
	}{
		{name: "negative", count: -1, want: ""},
		{name: "zero", count: 0, want: ""},
		{name: "single digit", count: 3, want: "3"},
		{name: "two digits", count: 18, want: "18"},
		{name: "maximum", count: 99, want: "99"},
		{name: "overflow", count: 100, want: "99+"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := FormatUnreadCount(test.count); got != test.want {
				t.Fatalf("FormatUnreadCount(%d) = %q, want %q", test.count, got, test.want)
			}
		})
	}
}
