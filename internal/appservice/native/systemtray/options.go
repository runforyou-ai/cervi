//go:build !server

package systemtray

import "github.com/wailsapp/wails/v3/pkg/application"

// Options 定义原生系统托盘初始化所需的应用资源与回调。
type Options struct {
	App             *application.App
	Window          application.Window
	Icon            []byte
	MacTemplateIcon []byte
	RequestQuit     func()
}
