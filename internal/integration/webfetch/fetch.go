//go:build server

// Package webfetch 按地址抓取网页，供知识库导入网页内容。
package webfetch

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// maxResponseBytes 是单个页面允许读取的最大字节数。
	maxResponseBytes = 10 << 20
	// maxRedirects 是抓取过程中允许跟随的重定向次数。
	maxRedirects = 5
	// userAgent 标识抓取来自 Cervi 知识库导入。
	userAgent = "Cervi-KnowledgeImport/1.0"
	// htmlPageName 与 textPageName 是送转换服务的文件名，决定转换器的选择。
	htmlPageName = "page.html"
	textPageName = "page.txt"
)

// Error 定义网页抓取的语言无关失败原因码。
type Error struct {
	Code string `json:"code"`
}

// Error 返回语言无关的失败原因。
func (e *Error) Error() string { return "web fetch: " + e.Code }

// Page 返回送转换服务使用的文件名和页面内容。
type Page struct {
	Name string
	Body []byte
}

// Client 抓取单个公开网页。
type Client struct{ http *http.Client }

// NewClient 创建网页抓取客户端。
func NewClient() *Client {
	return &Client{http: &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("too many redirects")
			}
			return nil
		},
	}}
}

// Normalize 校验并规范化页面地址，只接受 http 与 https 的绝对地址并去掉片段。
func Normalize(target string) (string, error) {
	parsed, err := parseTarget(target)
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}

// parseTarget 解析页面地址并去掉片段。
func parseTarget(target string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(target))
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return nil, &Error{Code: "url_invalid"}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, &Error{Code: "url_invalid"}
	}
	parsed.Fragment = ""
	parsed.RawFragment = ""
	return parsed, nil
}

// Fetch 读取页面内容，按响应内容类型决定送转换服务的文件名。
func (c *Client) Fetch(ctx context.Context, target string) (Page, error) {
	parsed, err := parseTarget(target)
	if err != nil {
		return Page{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return Page{}, &Error{Code: "url_unreachable"}
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain")
	response, err := c.http.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return Page{}, ctx.Err()
		}
		return Page{}, &Error{Code: "url_unreachable"}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return Page{}, &Error{Code: "url_unreachable"}
	}
	name, err := documentName(response.Header.Get("Content-Type"))
	if err != nil {
		return Page{}, err
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return Page{}, &Error{Code: "url_unreachable"}
	}
	if len(body) > maxResponseBytes {
		return Page{}, &Error{Code: "url_content_too_large"}
	}
	body = decodeUTF8(body, response.Header.Get("Content-Type"))
	if name == htmlPageName {
		body = extractArticle(body, parsed)
	}
	return Page{Name: name, Body: body}, nil
}

// documentName 按响应内容类型返回转换服务识别的文件名。
func documentName(contentType string) (string, error) {
	media, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", &Error{Code: "url_content_unsupported"}
	}
	switch media {
	case "text/html", "application/xhtml+xml":
		return htmlPageName, nil
	case "text/plain":
		return textPageName, nil
	default:
		return "", &Error{Code: "url_content_unsupported"}
	}
}
