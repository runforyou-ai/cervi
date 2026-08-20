//go:build server

package publicweb

import _ "embed"

// widgetScript 是网站渠道嵌入脚本。
//
//go:embed widget.js
var widgetScript []byte
