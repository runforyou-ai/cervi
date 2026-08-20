//go:build server

package publicweb

import _ "embed"

// widgetScript 是网站渠道嵌入脚本。
//
//go:embed widget.js
var widgetScript []byte

// chromeCSS 是访客聊天页样式。
//
//go:embed chrome.css
var chromeCSS string

// pageHTML 是访客聊天页模板。
//
//go:embed page.html
var pageHTML string

// chatJS 是访客聊天页本机演示脚本。
//
//go:embed chat.js
var chatJS string
