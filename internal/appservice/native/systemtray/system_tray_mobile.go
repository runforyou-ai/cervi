//go:build !server && (android || ios)

package systemtray

import (
	"github.com/runforyou-ai/cervi/internal/appservice"
)

// Setup 保持移动端不启用桌面托盘能力。
func Setup(_ Options) appservice.UnreadIndicator {
	return &noopUnreadIndicator{}
}
