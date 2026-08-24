//go:build !server

package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const unreadIndicatorDemoInterval = 3 * time.Second

var unreadIndicatorDemoCounts = [...]int{3, 18, 128, 0}

// startUnreadIndicatorDemo 临时循环展示未读角标；跨平台验证完成后应停止调用。
func startUnreadIndicatorDemo(app *application.App, indicator appservice.UnreadIndicator) {
	var once sync.Once
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		once.Do(func() {
			go runUnreadIndicatorDemo(app.Context(), indicator)
		})
	})
}

// runUnreadIndicatorDemo 依次展示个位数、两位数、上限文本和清除状态。
func runUnreadIndicatorDemo(ctx context.Context, indicator appservice.UnreadIndicator) {
	index := 0
	update := func() {
		count := unreadIndicatorDemoCounts[index]
		if err := indicator.SetUnreadCount(count); err != nil {
			slog.Warn("更新临时未读消息角标失败", "count", count, "error", err)
		}
		index = (index + 1) % len(unreadIndicatorDemoCounts)
	}

	update()
	ticker := time.NewTicker(unreadIndicatorDemoInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			update()
		case <-ctx.Done():
			return
		}
	}
}
