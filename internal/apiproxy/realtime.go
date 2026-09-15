//go:build !server

package apiproxy

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/appservice"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// realtimeIdleTimeout 是原生端未收到任何服务端事件即断开事件流的时限，服务端每 25 秒发送心跳。
const realtimeIdleTimeout = 60 * time.Second

// realtimeConnectTimeout 是原生端等待事件流响应头的时限。
var realtimeConnectTimeout = 30 * time.Second

// realtimeClient 持有原生端到企业服务器的唯一实时事件流，并把服务端事件经 Wails 事件交给前端。
type realtimeClient struct {
	emit    func(name string, data any)
	mu      sync.Mutex
	current *realtimeSession
}

// realtimeSession 是一次实时事件流请求，接收协程独占读取响应。
type realtimeSession struct {
	id     string
	cancel context.CancelFunc
}

// ConnectRealtime 使用当前登录凭据建立实时事件流，替换已有事件流。
func (b *Backend) ConnectRealtime(ctx context.Context, meta appservice.RequestMeta) (appservice.RealtimeConnection, error) {
	state := b.connection.currentState()
	if state == nil {
		return appservice.RealtimeConnection{}, appservice.SessionError(meta, appservice.SessionStateConnect, cervii18n.ErrorServerConnectionRequired)
	}
	credential, authenticated := b.sessions.Current(ctx, state.baseURL.String())
	if !authenticated {
		return appservice.RealtimeConnection{}, appservice.SessionError(meta, appservice.SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	// 响应头返回前调用取消会中止请求，之后事件流独立于本次调用。
	streamCtx, cancel := context.WithCancel(context.Background())
	stopCancel := context.AfterFunc(ctx, cancel)
	defer stopCancel()
	request, err := http.NewRequestWithContext(streamCtx, http.MethodGet, remoteEndpoint(state.baseURL, "/realtime", ""), nil)
	if err != nil {
		cancel()
		return appservice.RealtimeConnection{}, appservice.FailedError(meta, cervii18n.ErrorRemoteRequestCreateFailed)
	}
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Accept-Language", string(meta.Locale))
	request.Header.Set("Authorization", "Bearer "+credential.Token)
	// 事件流是长响应，使用不设整体超时的客户端，只限制等待响应头的时间。
	connectTimer := time.AfterFunc(realtimeConnectTimeout, cancel)
	response, err := (&http.Client{Transport: state.client.Transport}).Do(request)
	connectTimer.Stop()
	if err != nil {
		cancel()
		if ctx.Err() != nil {
			return appservice.RealtimeConnection{}, ctx.Err()
		}
		slog.Warn("建立实时事件流失败", "server_url", state.baseURL.String(), "error", err)
		return appservice.RealtimeConnection{}, appservice.UnavailableError(meta, cervii18n.ErrorServerConnectionFailed, nil)
	}
	if response.StatusCode != http.StatusOK {
		defer cancel()
		defer response.Body.Close()
		return appservice.RealtimeConnection{}, b.remoteError(ctx, state, &credential, response, http.MethodGet, "/realtime")
	}
	// 请求期间登录会话或企业服务器已变化时丢弃新事件流，由前端按新凭据重连。
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	if current, ok := b.sessions.Current(ctx, state.baseURL.String()); !ok || current.Token != credential.Token || b.connection.currentState() != state {
		cancel()
		response.Body.Close()
		slog.Info("登录会话在建立实时事件流期间变化，丢弃新事件流", "server_url", state.baseURL.String())
		return appservice.RealtimeConnection{}, appservice.UnavailableError(meta, cervii18n.ErrorServerConnectionFailed, nil)
	}
	session := b.realtime.start(response.Body, cancel)
	slog.Info("实时事件流已建立", "connection_id", session.id, "server_url", state.baseURL.String(), "user_id", credential.UserID)
	return appservice.RealtimeConnection{ConnectionID: session.id}, nil
}

// DisconnectRealtime 关闭原生端当前实时事件流。
func (b *Backend) DisconnectRealtime(context.Context, appservice.RequestMeta) error {
	b.realtime.disconnect()
	return nil
}

// start 登记新事件流并启动接收协程，已有事件流随之关闭。
func (c *realtimeClient) start(body io.ReadCloser, cancel context.CancelFunc) *realtimeSession {
	session := &realtimeSession{id: uuid.NewV7().String(), cancel: cancel}
	c.mu.Lock()
	previous := c.current
	c.current = session
	c.mu.Unlock()
	if previous != nil {
		previous.cancel()
	}
	go c.receive(session, body)
	return session
}

// disconnect 解除当前事件流登记并关闭，接收协程随后投递结束事件。
func (c *realtimeClient) disconnect() {
	c.mu.Lock()
	session := c.current
	c.current = nil
	c.mu.Unlock()
	if session != nil {
		session.cancel()
	}
}

// receive 逐行读取事件流并把每条 data 事件原文投递给前端；超过空闲时限未读到任何行时断开，结束时投递结束事件。
func (c *realtimeClient) receive(session *realtimeSession, body io.ReadCloser) {
	defer body.Close()
	idle := time.AfterFunc(realtimeIdleTimeout, session.cancel)
	defer idle.Stop()
	scanner := bufio.NewScanner(body)
	scanner.Buffer(nil, maxResponseBytes)
	for scanner.Scan() {
		idle.Reset(realtimeIdleTimeout)
		// 服务端每条事件只有一行 data。
		if frame, ok := strings.CutPrefix(scanner.Text(), "data: "); ok {
			c.emit(appservice.RealtimeFrameEventName, appservice.RealtimeFrameEvent{ConnectionID: session.id, Frame: frame})
		}
	}
	session.cancel()
	c.mu.Lock()
	if c.current == session {
		c.current = nil
	}
	c.mu.Unlock()
	slog.Info("实时事件流已结束", "connection_id", session.id, "error", scanner.Err())
	c.emit(appservice.RealtimeClosedEventName, appservice.RealtimeClosedEvent{ConnectionID: session.id})
}
