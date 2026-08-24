//go:build !server && windows

package systemtray

import (
	"context"
	"sync"
	"time"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const windowsTrayFlashInterval = 500 * time.Millisecond

// windowsUnreadIndicator 根据未读状态控制 Windows 通知区域图标显隐闪烁。
type windowsUnreadIndicator struct {
	ctx     context.Context
	tray    *application.SystemTray
	changed chan struct{}

	mu        sync.Mutex
	hasUnread bool
}

// newUnreadIndicator 创建 Windows 未读消息指示器并启动状态循环。
func newUnreadIndicator(app *application.App, tray *application.SystemTray) appservice.UnreadIndicator {
	indicator := &windowsUnreadIndicator{
		ctx:     app.Context(),
		tray:    tray,
		changed: make(chan struct{}, 1),
	}
	go indicator.run()
	return indicator
}

// SetUnreadCount 根据是否存在未读消息启动或停止托盘图标显隐闪烁。
func (i *windowsUnreadIndicator) SetUnreadCount(count int) error {
	i.mu.Lock()
	i.hasUnread = count > 0
	i.mu.Unlock()

	select {
	case i.changed <- struct{}{}:
	default:
	}
	return nil
}

// run 串行更新托盘图标，保证未读状态切换不会创建重复闪烁循环。
func (i *windowsUnreadIndicator) run() {
	var ticker *time.Ticker
	var ticks <-chan time.Time
	visible := true

	stopFlashing := func() {
		if ticker != nil {
			ticker.Stop()
			ticker = nil
			ticks = nil
		}
		visible = true
		i.tray.Show()
	}

	for {
		select {
		case <-i.changed:
			i.mu.Lock()
			hasUnread := i.hasUnread
			i.mu.Unlock()

			if !hasUnread {
				stopFlashing()
				continue
			}
			if ticker == nil {
				ticker = time.NewTicker(windowsTrayFlashInterval)
				ticks = ticker.C
				visible = false
				i.tray.Hide()
			}
		case <-ticks:
			visible = !visible
			if visible {
				i.tray.Show()
			} else {
				i.tray.Hide()
			}
		case <-i.ctx.Done():
			if ticker != nil {
				ticker.Stop()
			}
			return
		}
	}
}
