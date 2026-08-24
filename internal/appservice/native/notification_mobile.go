//go:build !server && (android || ios)

package native

import (
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// NewNotificationProvider 禁用移动端原生通知能力。
func NewNotificationProvider() (appservice.NativeNotification, []application.Service) {
	return nil, nil
}
