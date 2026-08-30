package tenant

import "context"

type accessHostContextKey struct{}

// WithAccessHost 把规范化后的请求访问地址写入上下文。
func WithAccessHost(ctx context.Context, accessHost string) context.Context {
	return context.WithValue(ctx, accessHostContextKey{}, accessHost)
}

// AccessHost 返回当前请求的规范化访问地址。
func AccessHost(ctx context.Context) string {
	accessHost, _ := ctx.Value(accessHostContextKey{}).(string)
	return accessHost
}
