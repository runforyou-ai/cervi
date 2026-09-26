//go:build server

package api

import "net/http"

// OfficialLoginCallbackPath 是企业域名下接收官方账号授权码的路径。
const OfficialLoginCallbackPath = "/auth/callback"

// OfficialLoginCallbackMiddleware 把官方账号登录回调转到前端 hash 路由的回调页，并原样保留授权响应参数。
func OfficialLoginCallbackMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet && request.URL.Path == OfficialLoginCallbackPath {
			http.Redirect(writer, request, "/#"+OfficialLoginCallbackPath+"?"+request.URL.RawQuery, http.StatusFound)
			return
		}
		next.ServeHTTP(writer, request)
	})
}
