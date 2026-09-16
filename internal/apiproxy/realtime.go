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

// realtimeClient 持有原生端到企业服务器的成员事件流与按运行编号建立的运行过程流，并把服务端事件经 Wails 事件交给前端。
type realtimeClient struct {
	emit func(name string, data any)
	mu   sync.Mutex
	// generationValue 在整体断开时递增，期间已建立的事件流登记时按旧会话丢弃。
	generationValue int
	current         *realtimeSession
	runs            map[string]*realtimeSession
}

// realtimeSession 是一次实时事件流请求，接收协程独占读取响应。
type realtimeSession struct {
	id           string
	runID        string
	cancel       context.CancelFunc
	releaseAfter func(*realtimeSession)
	// emitFrame 与 emitClosed 按事件流类型投递事件。
	emitFrame  func(*realtimeSession, string)
	emitClosed func(*realtimeSession)
}

// ConnectRealtime 使用当前登录凭据建立成员实时事件流，替换已有事件流。
func (b *Backend) ConnectRealtime(ctx context.Context, meta appservice.RequestMeta) (appservice.RealtimeConnection, error) {
	generation := b.realtime.generation()
	response, cancel, err := b.openEventStream(ctx, meta, "/realtime")
	if err != nil {
		return appservice.RealtimeConnection{}, err
	}
	session, ok := b.realtime.start(response.Body, cancel, generation)
	if !ok {
		return appservice.RealtimeConnection{}, staleEventStream(meta, cancel, response)
	}
	slog.Info("实时事件流已建立", "connection_id", session.id, "protocol", response.Proto)
	return appservice.RealtimeConnection{ConnectionID: session.id}, nil
}

// DisconnectRealtime 关闭原生端当前成员实时事件流及其登录会话下的全部运行过程流。
func (b *Backend) DisconnectRealtime(context.Context, appservice.RequestMeta) error {
	b.realtime.disconnect()
	return nil
}

// ConnectAgentRunStream 使用当前登录凭据建立指定运行的过程流，同一运行可重复请求，各自独立。
func (b *Backend) ConnectAgentRunStream(ctx context.Context, meta appservice.RequestMeta, runID string) (appservice.RealtimeConnection, error) {
	generation := b.realtime.generation()
	response, cancel, err := b.openEventStream(ctx, meta, "/realtime/runs/"+runID)
	if err != nil {
		return appservice.RealtimeConnection{}, err
	}
	session, ok := b.realtime.startRun(response.Body, cancel, runID, generation)
	if !ok {
		return appservice.RealtimeConnection{}, staleEventStream(meta, cancel, response)
	}
	slog.Info("运行过程流已建立", "connection_id", session.id, "agent_run_id", runID, "protocol", response.Proto)
	return appservice.RealtimeConnection{ConnectionID: session.id}, nil
}

// staleEventStream 关闭建立期间已被整体断开的事件流，由前端按新凭据重新请求。
func staleEventStream(meta appservice.RequestMeta, cancel context.CancelFunc, response *http.Response) error {
	cancel()
	response.Body.Close()
	slog.Info("实时通道在建立事件流期间整体断开，丢弃新事件流")
	return appservice.UnavailableError(meta, cervii18n.ErrorServerConnectionFailed, nil)
}

// generation 返回当前会话代次。
func (c *realtimeClient) generation() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generationValue
}

// DisconnectAgentRunStream 关闭指定本地流编号的运行过程流。
func (b *Backend) DisconnectAgentRunStream(_ context.Context, _ appservice.RequestMeta, connectionID string) error {
	b.realtime.disconnectRun(connectionID)
	return nil
}

// openEventStream 使用当前登录凭据向企业服务器发起事件流请求，返回响应与结束该流的取消函数。
func (b *Backend) openEventStream(ctx context.Context, meta appservice.RequestMeta, path string) (*http.Response, context.CancelFunc, error) {
	state := b.connection.currentState()
	if state == nil {
		return nil, nil, appservice.SessionError(meta, appservice.SessionStateConnect, cervii18n.ErrorServerConnectionRequired)
	}
	credential, authenticated := b.sessions.Current(ctx, state.baseURL.String())
	if !authenticated {
		return nil, nil, appservice.SessionError(meta, appservice.SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	// 响应头返回前调用取消会中止请求，之后事件流独立于本次调用。
	streamCtx, cancel := context.WithCancel(context.Background())
	stopCancel := context.AfterFunc(ctx, cancel)
	defer stopCancel()
	request, err := http.NewRequestWithContext(streamCtx, http.MethodGet, remoteEndpoint(state.baseURL, path, ""), nil)
	if err != nil {
		cancel()
		return nil, nil, appservice.FailedError(meta, cervii18n.ErrorRemoteRequestCreateFailed)
	}
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Accept-Language", string(meta.Locale))
	request.Header.Set("Authorization", "Bearer "+credential.Token)
	// 事件流是长响应，使用不设整体超时的客户端，只限制等待响应头的时间。
	connectTimer := time.AfterFunc(realtimeConnectTimeout, cancel)
	response, err := (&http.Client{Transport: state.client.Transport}).Do(request)
	// 计时器已触发时请求已被取消，按建立超时处理。
	if !connectTimer.Stop() && err == nil {
		response.Body.Close()
		err = context.DeadlineExceeded
	}
	if err != nil {
		cancel()
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		slog.Warn("建立实时事件流失败", "server_url", state.baseURL.String(), "path", path, "error", err)
		return nil, nil, appservice.UnavailableError(meta, cervii18n.ErrorServerConnectionFailed, nil)
	}
	if response.StatusCode != http.StatusOK {
		defer cancel()
		defer response.Body.Close()
		return nil, nil, b.remoteError(ctx, state, &credential, response, http.MethodGet, path)
	}
	// 请求期间登录会话或企业服务器已变化时丢弃新事件流，由前端按新凭据重连。
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	if current, ok := b.sessions.Current(ctx, state.baseURL.String()); !ok || current.Token != credential.Token || b.connection.currentState() != state {
		cancel()
		response.Body.Close()
		slog.Info("登录会话在建立实时事件流期间变化，丢弃新事件流", "server_url", state.baseURL.String(), "path", path)
		return nil, nil, appservice.UnavailableError(meta, cervii18n.ErrorServerConnectionFailed, nil)
	}
	return response, cancel, nil
}

// start 登记新的成员事件流并启动接收协程，已有事件流随之关闭；会话代次已变化时不登记并返回 false。
func (c *realtimeClient) start(body io.ReadCloser, cancel context.CancelFunc, generation int) (*realtimeSession, bool) {
	session := &realtimeSession{id: uuid.NewV7().String(), cancel: cancel}
	session.emitFrame = func(current *realtimeSession, frame string) {
		c.emit(appservice.RealtimeFrameEventName, appservice.RealtimeFrameEvent{ConnectionID: current.id, Frame: frame})
	}
	session.emitClosed = func(current *realtimeSession) {
		c.emit(appservice.RealtimeClosedEventName, appservice.RealtimeClosedEvent{ConnectionID: current.id})
	}
	session.releaseAfter = func(ended *realtimeSession) {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.current == ended {
			c.current = nil
		}
	}
	c.mu.Lock()
	if c.generationValue != generation {
		c.mu.Unlock()
		return nil, false
	}
	previous := c.current
	c.current = session
	c.mu.Unlock()
	if previous != nil {
		previous.cancel()
	}
	go c.receive(session, body)
	return session, true
}

// disconnect 关闭当前成员事件流与全部运行过程流；成员事件流断开即当前登录会话的实时通道整体结束。
func (c *realtimeClient) disconnect() {
	c.mu.Lock()
	c.generationValue++
	sessions := make([]*realtimeSession, 0, len(c.runs)+1)
	if c.current != nil {
		sessions = append(sessions, c.current)
	}
	for _, session := range c.runs {
		sessions = append(sessions, session)
	}
	c.current, c.runs = nil, nil
	c.mu.Unlock()
	for _, session := range sessions {
		session.cancel()
	}
}

// startRun 登记新的运行过程流并启动接收协程，多条运行过程流同时存在；会话代次已变化时不登记并返回 false。
func (c *realtimeClient) startRun(body io.ReadCloser, cancel context.CancelFunc, runID string, generation int) (*realtimeSession, bool) {
	session := &realtimeSession{id: uuid.NewV7().String(), runID: runID, cancel: cancel}
	session.emitFrame = func(current *realtimeSession, frame string) {
		c.emit(appservice.RealtimeRunFrameEventName, appservice.RealtimeRunFrameEvent{ConnectionID: current.id, RunID: current.runID, Frame: frame})
	}
	session.emitClosed = func(current *realtimeSession) {
		c.emit(appservice.RealtimeRunClosedEventName, appservice.RealtimeRunClosedEvent{ConnectionID: current.id, RunID: current.runID})
	}
	session.releaseAfter = func(ended *realtimeSession) {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.runs[ended.id] == ended {
			delete(c.runs, ended.id)
		}
	}
	c.mu.Lock()
	if c.generationValue != generation {
		c.mu.Unlock()
		return nil, false
	}
	if c.runs == nil {
		c.runs = map[string]*realtimeSession{}
	}
	c.runs[session.id] = session
	c.mu.Unlock()
	go c.receive(session, body)
	return session, true
}

// disconnectRun 解除指定运行过程流登记并关闭，接收协程随后投递结束事件。
func (c *realtimeClient) disconnectRun(connectionID string) {
	c.mu.Lock()
	session := c.runs[connectionID]
	delete(c.runs, connectionID)
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
			session.emitFrame(session, frame)
		}
	}
	session.cancel()
	session.releaseAfter(session)
	slog.Info("实时事件流已结束", "connection_id", session.id, "agent_run_id", session.runID, "error", scanner.Err())
	session.emitClosed(session)
}
