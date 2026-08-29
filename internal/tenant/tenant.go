// Package tenant 定义请求所属企业的解析边界。
package tenant

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
)

var (
	// ErrNotFound 表示当前请求没有匹配到企业。
	ErrNotFound = errors.New("tenant not found")
	// ErrAmbiguous 表示当前请求无法唯一匹配企业。
	ErrAmbiguous = errors.New("tenant is ambiguous")
)

// Scope 表示一次请求已经解析出的企业范围。
type Scope struct {
	OrganizationID   string
	OrganizationName string
}

// Resolver 根据规范化域名解析请求所属企业。
type Resolver interface {
	Resolve(context.Context, string) (Scope, error)
}

type hostnameContextKey struct{}

// HTTPMiddleware 把规范化后的请求域名写入请求上下文。
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctx := context.WithValue(request.Context(), hostnameContextKey{}, NormalizeHostname(request.Host))
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

// Hostname 返回当前请求的规范化域名。
func Hostname(ctx context.Context) string {
	hostname, _ := ctx.Value(hostnameContextKey{}).(string)
	return hostname
}

// NormalizeHostname 去除端口、末尾点和大小写差异，得到租户解析与 TLS 共用的域名键。
func NormalizeHostname(value string) string {
	hostname := strings.TrimSpace(value)
	if parsed, _, err := net.SplitHostPort(hostname); err == nil {
		hostname = parsed
	}
	hostname = strings.Trim(hostname, "[]")
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
}
