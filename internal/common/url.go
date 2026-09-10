package common

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ValidHTTPBaseURL 校验地址为不含认证信息、查询和片段的完整 HTTP 地址，用于 API 端点。
func ValidHTTPBaseURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil && parsed.IsAbs() && parsed.Host != "" && parsed.User == nil &&
		(parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.RawQuery == "" && parsed.Fragment == ""
}

// ValidHTTPURL 校验地址为不含认证信息的完整 HTTP 地址，允许查询和片段，用于浏览器打开的页面地址。
func ValidHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.IsAbs() && parsed.Host != "" && parsed.User == nil &&
		(strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https"))
}

// CompatibleModelBaseURL 把供应商地址规范为 OpenAI 兼容入口。
func CompatibleModelBaseURL(brand, value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("parse model base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("model base URL must include scheme and host")
	}
	if brand != "alibaba" {
		return strings.TrimSuffix(parsed.String(), "/"), nil
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	switch {
	case strings.HasSuffix(path, "/compatible-mode/v1"):
	case strings.HasSuffix(path, "/api/v1"):
		path = strings.TrimSuffix(path, "/api/v1") + "/compatible-mode/v1"
	default:
		path += "/compatible-mode/v1"
	}
	parsed.Path = path
	parsed.RawPath = ""
	return parsed.String(), nil
}
