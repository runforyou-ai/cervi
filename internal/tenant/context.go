package tenant

import "context"

type hostnameContextKey struct{}

// WithHostname 把规范化后的请求域名写入上下文。
func WithHostname(ctx context.Context, hostname string) context.Context {
	return context.WithValue(ctx, hostnameContextKey{}, hostname)
}

// Hostname 返回当前请求的规范化域名。
func Hostname(ctx context.Context) string {
	hostname, _ := ctx.Value(hostnameContextKey{}).(string)
	return hostname
}
