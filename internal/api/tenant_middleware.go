//go:build server

package api

import (
	"net/http"

	"github.com/runforyou-ai/cervi/internal/tenant"
)

// TenantContextMiddleware 把规范化后的请求域名写入租户上下文。
// 它应挂载在 Wails Assets.Middleware，确保 API、文件路由和绑定调用共享同一上下文。
func TenantContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctx := tenant.WithHostname(request.Context(), tenant.NormalizeHostname(request.Host))
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}
