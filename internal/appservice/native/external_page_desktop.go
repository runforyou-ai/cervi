//go:build !server && ((darwin && !ios) || windows || (linux && !android))

package native

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// externalPageOpener 在桌面端用独立窗口打开外部页面，并按地址复用已开窗口。
type externalPageOpener struct {
	mu      sync.Mutex
	windows map[string]*application.WebviewWindow
}

// NewExternalPageOpener 创建桌面端外部页面窗口能力。
func NewExternalPageOpener() appservice.ExternalPageOpener {
	return &externalPageOpener{windows: make(map[string]*application.WebviewWindow)}
}

// OpenExternalPage 打开外部页面窗口；同一地址已打开时聚焦现有窗口。
func (o *externalPageOpener) OpenExternalPage(_ context.Context, _ appservice.RequestMeta, input appservice.ExternalPageInput) error {
	app := application.Get()
	if app == nil {
		return errors.New("application not running")
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	if window, ok := o.windows[input.URL]; ok {
		window.Restore()
		window.Focus()
		slog.Info("已聚焦外部页面窗口", "title", input.Title, "url", input.URL)
		return nil
	}

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            input.Title,
		Width:            1280,
		Height:           800,
		MinWidth:         640,
		MinHeight:        480,
		BackgroundColour: application.NewRGB(250, 250, 250),
		URL:              input.URL,
	})
	o.windows[input.URL] = window
	window.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		o.mu.Lock()
		delete(o.windows, input.URL)
		o.mu.Unlock()
	})
	slog.Info("已打开外部页面窗口", "title", input.Title, "url", input.URL)
	return nil
}
