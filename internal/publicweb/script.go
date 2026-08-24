//go:build server

package publicweb

import _ "embed"

// composerEmojisJSON 是访客 Messenger 的固定表情候选。
//
//go:embed composer-emojis.json
var composerEmojisJSON string

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

// previewHTML 是管理端挂件预览宿主页模板。
//
//go:embed preview.html
var previewHTML string

// chatJS 是访客聊天页交互脚本。
//
//go:embed chat.js
var chatJS string
