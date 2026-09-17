//go:build server

package webfetch

import (
	"bytes"
	"io"
	"net/url"
	"strings"

	"github.com/go-shiori/go-readability"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

const (
	// articlePrefix 与 articleSuffix 把提取出的正文包成完整 HTML 文档，供转换服务识别。
	articlePrefix = `<!DOCTYPE html><html><head><meta charset="utf-8"></head><body>`
	articleSuffix = `</body></html>`
)

// decodeUTF8 按 BOM、页面内声明和响应头判定页面编码并转成 UTF-8，判定失败时保留原字节。
func decodeUTF8(body []byte, contentType string) []byte {
	reader, err := charset.NewReader(bytes.NewReader(body), contentType)
	if err != nil {
		return body
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return body
	}
	return decoded
}

// extractArticle 提取 HTML 页面的正文主体，去掉导航、侧栏和页脚等页面结构；提取不到正文时返回整页。
func extractArticle(page []byte, pageURL *url.URL) []byte {
	document, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return page
	}
	article, err := readability.FromDocument(document, pageURL)
	if err != nil || strings.TrimSpace(article.TextContent) == "" {
		return page
	}
	return []byte(articlePrefix + article.Content + articleSuffix)
}
