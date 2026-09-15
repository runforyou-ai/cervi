//go:build !server

package apiproxy

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"sync"
	"time"
	"uuid"

	"github.com/coder/websocket"
	"github.com/runforyou-ai/cervi/internal/appservice"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/realtime/protocol"
)

// realtimePingInterval 是原生端发送心跳帧的间隔。
const realtimePingInterval = 25 * time.Second

// realtimeWriteTimeout 是原生端写入单帧的上限。
const realtimeWriteTimeout = 10 * time.Second

// realtimeClient 持有原生端到企业服务器的唯一实时连接，并把服务端帧经事件交给前端。
type realtimeClient struct {
	emit    func(name string, data any)
	mu      sync.Mutex
	current *realtimeSession
}

// realtimeSession 是一次实时连接，心跳协程独占写入，接收协程独占读取。
type realtimeSession struct {
	id     string
	socket *websocket.Conn
	cancel context.CancelFunc
}

// ConnectRealtime 使用当前登录凭据建立实时连接并发送认证与 Hello 帧，替换已有连接。
func (b *Backend) ConnectRealtime(ctx context.Context, meta appservice.RequestMeta, input appservice.RealtimeConnectInput) (appservice.RealtimeConnection, error) {
	state := b.connection.currentState()
	if state == nil {
		return appservice.RealtimeConnection{}, appservice.SessionError(meta, appservice.SessionStateConnect, cervii18n.ErrorServerConnectionRequired)
	}
	credential, authenticated := b.sessions.Current(ctx, state.baseURL.String())
	if !authenticated {
		return appservice.RealtimeConnection{}, appservice.SessionError(meta, appservice.SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	b.realtime.disconnect()
	socket, _, err := websocket.Dial(ctx, remoteEndpoint(state.baseURL, "/realtime", ""), &websocket.DialOptions{HTTPClient: state.client})
	if err != nil {
		if ctx.Err() != nil {
			return appservice.RealtimeConnection{}, ctx.Err()
		}
		slog.Warn("建立实时连接失败", "server_url", state.baseURL.String(), "error", err)
		return appservice.RealtimeConnection{}, appservice.UnavailableError(meta, cervii18n.ErrorServerConnectionFailed, nil)
	}
	socket.SetReadLimit(maxResponseBytes)
	clientKind := protocol.ClientDesktop
	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		clientKind = protocol.ClientMobile
	}
	session := b.realtime.start(socket,
		protocol.Authenticate{Token: credential.Token},
		protocol.ClientHello{ClientKind: clientKind, AppVersion: input.AppVersion, Capabilities: input.Capabilities},
	)
	slog.Info("实时连接已建立", "connection_id", session.id, "server_url", state.baseURL.String(), "user_id", credential.UserID)
	return appservice.RealtimeConnection{ConnectionID: session.id}, nil
}

// DisconnectRealtime 关闭原生端当前实时连接。
func (b *Backend) DisconnectRealtime(context.Context, appservice.RequestMeta) error {
	b.realtime.disconnect()
	return nil
}

// start 登记新连接并启动收发协程，已有连接在后台关闭。
func (c *realtimeClient) start(socket *websocket.Conn, initial ...protocol.Frame) *realtimeSession {
	ctx, cancel := context.WithCancel(context.Background())
	session := &realtimeSession{id: uuid.NewV7().String(), socket: socket, cancel: cancel}
	c.mu.Lock()
	previous := c.current
	c.current = session
	c.mu.Unlock()
	if previous != nil {
		go previous.close()
	}
	go c.receive(ctx, session)
	go session.heartbeat(ctx, initial)
	return session
}

// disconnect 解除当前连接登记并在后台关闭，接收协程随后投递关闭事件。
func (c *realtimeClient) disconnect() {
	c.mu.Lock()
	session := c.current
	c.current = nil
	c.mu.Unlock()
	if session != nil {
		go session.close()
	}
}

// receive 把服务端帧原样投递给前端，连接结束时投递关闭码与原因。
func (c *realtimeClient) receive(ctx context.Context, session *realtimeSession) {
	for {
		_, data, err := session.socket.Read(ctx)
		if err == nil {
			c.emit(appservice.RealtimeFrameEventName, appservice.RealtimeFrameEvent{ConnectionID: session.id, Frame: string(data)})
			continue
		}
		code, reason := -1, ""
		var closeError websocket.CloseError
		if errors.As(err, &closeError) {
			code, reason = int(closeError.Code), closeError.Reason
		}
		session.cancel()
		c.mu.Lock()
		if c.current == session {
			c.current = nil
		}
		c.mu.Unlock()
		slog.Info("实时连接已结束", "connection_id", session.id, "code", code, "reason", reason)
		c.emit(appservice.RealtimeClosedEventName, appservice.RealtimeClosedEvent{ConnectionID: session.id, Code: code, Reason: reason})
		return
	}
}

// heartbeat 先发送认证与 Hello 帧，之后按固定间隔发送 Ping，写入失败时连接随之关闭。
func (s *realtimeSession) heartbeat(ctx context.Context, frames []protocol.Frame) {
	ticker := time.NewTicker(realtimePingInterval)
	defer ticker.Stop()
	for {
		for _, frame := range frames {
			data, err := protocol.Encode(frame)
			if err != nil {
				slog.Warn("编码实时帧失败", "connection_id", s.id, "type", frame.FrameType(), "error", err)
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, realtimeWriteTimeout)
			err = s.socket.Write(writeCtx, websocket.MessageText, data)
			cancel()
			if err != nil {
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			frames = []protocol.Frame{protocol.Ping{}}
		}
	}
}

// close 以正常关闭码结束连接。
func (s *realtimeSession) close() {
	if err := s.socket.Close(websocket.StatusNormalClosure, ""); err != nil {
		slog.Info("实时连接关闭握手未完成", "connection_id", s.id, "error", err)
	}
	s.cancel()
}
