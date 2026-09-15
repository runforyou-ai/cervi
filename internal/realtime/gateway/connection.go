//go:build server

package gateway

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
	"uuid"

	"github.com/coder/websocket"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/realtime/protocol"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// mergeKey 标识发送队列中可按最高版本合并的变更通知帧。
type mergeKey struct {
	frameType      protocol.Type
	conversationID string
}

// connection 是一条成员实时连接，读协程处理客户端帧，写协程独占发送。
type connection struct {
	gateway  *Gateway
	id       string
	socket   *websocket.Conn
	cancel   context.CancelFunc
	lastRead atomic.Int64

	// subjects 与 tokenSessionID 在加入受众时写入，之后只读。
	subjects       []string
	tokenSessionID string

	mu     sync.Mutex
	queue  []protocol.Frame
	merged map[mergeKey]int
	// epoch 在清除队列时递增，写协程据此丢弃已取出但未发送的帧。
	epoch       int
	wake        chan struct{}
	closing     bool
	closeStatus websocket.StatusCode
	closeReason string
}

// newConnection 创建等待认证的连接。
func newConnection(gateway *Gateway, socket *websocket.Conn) *connection {
	return &connection{
		gateway: gateway,
		id:      uuid.NewV7().String(),
		socket:  socket,
		merged:  map[mergeKey]int{},
		wake:    make(chan struct{}, 1),
	}
}

// run 依次完成首帧认证、Hello 与心跳处理，直到连接关闭。
func (c *connection) run(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)
	defer c.cancel()
	written := make(chan struct{})
	go func() {
		defer close(written)
		c.write(ctx)
	}()
	// 服务端发起关闭时等待写协程发完结束帧；读流因对端断开结束时直接取消写协程。
	defer func() {
		c.mu.Lock()
		closing := c.closing
		c.mu.Unlock()
		if !closing {
			c.cancel()
		}
		<-written
	}()

	// 首帧须在认证时限内到达，定时器已触发时连接已进入关闭流程。
	authTimer := time.AfterFunc(c.gateway.options.AuthTimeout, func() {
		slog.Warn("实时连接认证超时", "connection_id", c.id)
		c.fail(protocol.ErrorAuthenticationTimeout)
	})
	frame, ok := c.read(ctx)
	if stopped := authTimer.Stop(); !ok || !stopped {
		return
	}
	authenticate, ok := frame.(protocol.Authenticate)
	if !ok {
		c.fail(protocol.ErrorInvalidFrame)
		return
	}
	identity, err := c.gateway.backend.AuthenticateMember(ctx, authenticate.Token)
	if err != nil {
		c.reject(err)
		return
	}
	c.send(protocol.Authenticated{})
	go c.watchIdle(ctx)

	// 连接最长存活时间不晚于登录会话到期。
	lifetime := min(c.gateway.options.MaxLifetime, time.Until(identity.Token.ExpiresAt))
	expiry := time.AfterFunc(lifetime, func() {
		c.close(true, websocket.StatusNormalClosure, string(protocol.CloseSessionExpired))
	})
	defer expiry.Stop()
	slog.Info("实时连接已认证", "connection_id", c.id, "organization_id", identity.Organization.ID, "user_id", identity.User.ID, "lifetime", lifetime)

	helloReceived := false
	for {
		frame, ok := c.read(ctx)
		if !ok {
			return
		}
		switch value := frame.(type) {
		case protocol.Ping:
			c.send(protocol.Pong{})
		case protocol.Pong:
		case protocol.ClientHello:
			if helloReceived {
				c.fail(protocol.ErrorInvalidFrame)
				return
			}
			helloReceived = true
			if err := c.hello(ctx, authenticate.Token, identity, value); err != nil {
				c.reject(err)
				return
			}
		default:
			c.fail(protocol.ErrorInvalidFrame)
			return
		}
	}
}

// hello 安装受众订阅，订阅生效后重新校验登录会话并读取同步探针值，回复 ServerHello。
func (c *connection) hello(ctx context.Context, token string, identity *servermodels.Identity, hello protocol.ClientHello) error {
	if err := c.gateway.subscribe(c, identity); err != nil {
		return err
	}
	// 认证与订阅生效之间提交的登出或停用由重新校验发现，之后的撤销经受众通知送达。
	if _, err := c.gateway.backend.AuthenticateMember(ctx, token); err != nil {
		return err
	}
	heads, err := c.gateway.backend.MemberSyncHeads(ctx, identity)
	if err != nil {
		return err
	}
	c.send(protocol.ServerHello{ConnectionID: c.id, SyncHeads: heads})
	slog.Info("实时连接已就绪", "connection_id", c.id, "user_id", identity.User.ID, "client_kind", hello.ClientKind, "app_version", hello.AppVersion)
	return nil
}

// reject 按校验错误关闭连接：登录会话无效时为认证失败，其余为服务不可用。
func (c *connection) reject(err error) {
	if appservice.SessionStateOf(err) != "" {
		slog.Warn("实时连接认证失败", "connection_id", c.id)
		c.fail(protocol.ErrorAuthenticationFailed)
		return
	}
	slog.Warn("实时连接处理失败", "connection_id", c.id, "error", err)
	c.fail(protocol.ErrorUnavailable)
}

// watchIdle 在空闲时限内未读到任何帧时关闭连接，直到连接结束。
func (c *connection) watchIdle(ctx context.Context) {
	timer := time.NewTimer(c.gateway.options.IdleTimeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			remaining := c.gateway.options.IdleTimeout - time.Since(time.Unix(0, c.lastRead.Load()))
			if remaining <= 0 {
				c.close(true, websocket.StatusNormalClosure, string(protocol.CloseIdleTimeout))
				return
			}
			timer.Reset(remaining)
		}
	}
}

// read 读取并解码一帧客户端帧；忽略未知帧，协议错误时关闭连接，连接结束时返回 false。
func (c *connection) read(ctx context.Context) (protocol.Frame, bool) {
	for {
		messageType, data, err := c.socket.Read(ctx)
		if err != nil {
			return nil, false
		}
		c.lastRead.Store(time.Now().UnixNano())
		if messageType != websocket.MessageText {
			c.fail(protocol.ErrorInvalidFrame)
			return nil, false
		}
		frame, err := protocol.DecodeClient(data)
		switch {
		case errors.Is(err, protocol.ErrUnknownFrame):
			continue
		case errors.Is(err, protocol.ErrUnsupportedVersion):
			c.fail(protocol.ErrorUnsupportedVersion)
			return nil, false
		case err != nil:
			c.fail(protocol.ErrorInvalidFrame)
			return nil, false
		}
		return frame, true
	}
}

// send 把帧加入发送队列；变更通知按会话与种类保留最高版本，队列溢出时按慢连接关闭。
func (c *connection) send(frame protocol.Frame) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing {
		return
	}
	key, version, mergeable := mergeTarget(frame)
	if index, exists := c.merged[key]; mergeable && exists {
		if _, current, _ := mergeTarget(c.queue[index]); version > current {
			c.queue[index] = frame
		}
		return
	}
	if len(c.queue) >= c.gateway.options.QueueSize {
		slog.Warn("实时连接发送队列溢出，按慢连接关闭", "connection_id", c.id, "queued", len(c.queue))
		c.beginClose(true, websocket.StatusTryAgainLater, string(protocol.CloseSlowConsumer))
		return
	}
	if mergeable {
		c.merged[key] = len(c.queue)
	}
	c.queue = append(c.queue, frame)
	c.signal()
}

// mergeTarget 返回变更通知帧的合并键与版本，其他帧不可合并。
func mergeTarget(frame protocol.Frame) (mergeKey, int64, bool) {
	switch value := frame.(type) {
	case protocol.ConversationChanged:
		return mergeKey{protocol.TypeConversationChanged, value.ConversationID}, value.Version, true
	case protocol.ConversationStateChanged:
		return mergeKey{protocol.TypeConversationStateChanged, value.ConversationID}, value.Version, true
	case protocol.IdentityProfileChanged:
		return mergeKey{frameType: protocol.TypeIdentityProfileChanged}, value.Version, true
	}
	return mergeKey{}, 0, false
}

// fail 发送连接级错误后以错误码为原因关闭连接。
func (c *connection) fail(code protocol.ErrorCode) {
	status := websocket.StatusPolicyViolation
	if code == protocol.ErrorUnavailable {
		status = websocket.StatusTryAgainLater
	}
	c.close(false, status, string(code), protocol.RealtimeError{Code: code})
}

// revoke 清除未发送的帧，发送登录会话撤销帧后关闭连接。
func (c *connection) revoke(reason protocol.SessionRevokedReason) {
	slog.Info("实时连接登录会话已撤销", "connection_id", c.id, "reason", reason)
	c.close(true, websocket.StatusPolicyViolation, string(protocol.CloseSessionRevoked), protocol.SessionRevoked{Reason: reason})
}

// close 停止接收新帧，按需清除未发送的帧并追加结束帧，由写协程发送后关闭连接。
func (c *connection) close(discard bool, status websocket.StatusCode, reason string, frames ...protocol.Frame) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing {
		return
	}
	c.beginClose(discard, status, reason, frames...)
}

// beginClose 在持有连接锁时进入关闭状态并唤醒写协程。
func (c *connection) beginClose(discard bool, status websocket.StatusCode, reason string, frames ...protocol.Frame) {
	c.closing = true
	c.closeStatus = status
	c.closeReason = reason
	if discard {
		c.queue = nil
		clear(c.merged)
		c.epoch++
	}
	c.queue = append(c.queue, frames...)
	c.signal()
}

// signal 唤醒写协程，已有待处理唤醒时直接返回。
func (c *connection) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// write 按入队顺序发送帧，进入关闭状态且队列发送完毕后关闭连接；写入超时说明对端停止读取，直接断开。
func (c *connection) write(ctx context.Context) {
	for {
		c.mu.Lock()
		frames, epoch := c.queue, c.epoch
		c.queue = nil
		clear(c.merged)
		c.mu.Unlock()

		for _, frame := range frames {
			c.mu.Lock()
			discarded := c.epoch != epoch
			c.mu.Unlock()
			if discarded {
				break
			}
			data, err := protocol.Encode(frame)
			if err != nil {
				slog.Warn("编码实时帧失败", "connection_id", c.id, "type", frame.FrameType(), "error", err)
				continue
			}
			if len(data) > c.gateway.options.MaxFrameBytes {
				slog.Warn("实时帧超过大小上限，已丢弃", "connection_id", c.id, "type", frame.FrameType(), "bytes", len(data))
				continue
			}
			writeCtx, cancel := context.WithTimeout(ctx, c.gateway.options.WriteTimeout)
			err = c.socket.Write(writeCtx, websocket.MessageText, data)
			cancel()
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					slog.Warn("实时连接写入超时，断开连接", "connection_id", c.id)
				}
				_ = c.socket.CloseNow()
				c.cancel()
				return
			}
		}

		c.mu.Lock()
		pending, closing, status, reason := len(c.queue), c.closing, c.closeStatus, c.closeReason
		c.mu.Unlock()
		if pending > 0 {
			continue
		}
		if closing {
			if err := c.socket.Close(status, reason); err != nil {
				slog.Info("实时连接关闭握手未完成", "connection_id", c.id, "reason", reason, "error", err)
			}
			c.cancel()
			return
		}
		select {
		case <-c.wake:
		case <-ctx.Done():
			return
		}
	}
}
