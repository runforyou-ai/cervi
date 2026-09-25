package toolchain

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"

	"golang.org/x/net/html"
)

// packageFileURL 在 PyPI 简单索引（PEP 503）的项目页中查找指定文件的下载地址，索引不可用或未收录该文件时返回下载失败。
func packageFileURL(ctx context.Context, client *http.Client, index, project, file string) (string, error) {
	ctx, cancel := context.WithTimeoutCause(ctx, downloadStallTimeout, errDownloadStalled)
	defer cancel()
	page, err := url.Parse(index + "/" + project + "/")
	if err != nil {
		return "", fmt.Errorf("parse package index: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, page.String(), nil)
	if err != nil {
		return "", fmt.Errorf("build package index request: %w", err)
	}
	request.Header.Set("Accept", "text/html")
	request.Header.Set("User-Agent", userAgent)
	response, err := client.Do(request)
	if err != nil {
		return "", &stepError{failure: FailureDownload, err: fmt.Errorf("read package index %s: %w", page, cmp.Or(context.Cause(ctx), err))}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", &stepError{failure: FailureDownload, err: fmt.Errorf("read package index %s: HTTP %d", page, response.StatusCode)}
	}
	tokens := html.NewTokenizer(response.Body)
	for {
		switch tokens.Next() {
		case html.ErrorToken:
			err := tokens.Err()
			if errors.Is(err, io.EOF) {
				return "", &stepError{failure: FailureDownload, err: fmt.Errorf("package index %s does not list %s", page, file)}
			}
			return "", &stepError{failure: FailureDownload, err: fmt.Errorf("read package index %s: %w", page, cmp.Or(context.Cause(ctx), err))}
		case html.StartTagToken:
			name, hasAttributes := tokens.TagName()
			if string(name) != "a" || !hasAttributes {
				continue
			}
			for {
				key, value, more := tokens.TagAttr()
				// 链接可以是相对索引页的地址，文件名之后的片段是索引给出的摘要。
				if string(key) == "href" {
					if link, err := page.Parse(string(value)); err == nil && path.Base(link.Path) == file {
						link.Fragment = ""
						return link.String(), nil
					}
				}
				if !more {
					break
				}
			}
		}
	}
}
