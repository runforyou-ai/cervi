//go:build !server

package apiproxy

import (
	"context"
	"net/http"
	"net/url"

	"github.com/runforyou-ai/cervi/internal/appservice"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// DeviceModelEndpoint 返回指定运行在企业服务器上的模型代理入口，以及为模型请求附加登录令牌与本机设备编号的传输层；meta 必须携带设备编号。
func (b *Backend) DeviceModelEndpoint(ctx context.Context, meta appservice.RequestMeta, runID string) (string, http.RoundTripper, error) {
	state := b.connection.currentState()
	if state == nil {
		return "", nil, appservice.SessionError(meta, appservice.SessionStateConnect, cervii18n.ErrorServerConnectionRequired)
	}
	credential, authenticated := b.sessions.Current(ctx, state.baseURL.String())
	if !authenticated {
		return "", nil, appservice.SessionError(meta, appservice.SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	base := state.client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	endpoint := remoteEndpoint(state.baseURL, "/agent-runs/"+url.PathEscape(runID)+"/model", "")
	return endpoint, &deviceModelTransport{base: base, token: credential.Token, deviceID: meta.DeviceID}, nil
}

// deviceModelTransport 把模型组件写入的供应商凭据请求头换成登录令牌与本机设备编号。
type deviceModelTransport struct {
	base     http.RoundTripper
	token    string
	deviceID string
}

// RoundTrip 复制请求并替换认证请求头后发出。
func (t *deviceModelTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	outgoing := request.Clone(request.Context())
	outgoing.Header.Del("X-Api-Key")
	outgoing.Header.Del("X-Goog-Api-Key")
	outgoing.Header.Set("Authorization", "Bearer "+t.token)
	outgoing.Header.Set(appservice.DeviceHeader, t.deviceID)
	return t.base.RoundTrip(outgoing)
}
