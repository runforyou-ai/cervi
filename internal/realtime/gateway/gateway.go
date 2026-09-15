//go:build server

// Package gateway 在企业服务端内接收成员实时 WebSocket 连接，按已认证身份订阅受众通知并转发为实时帧。
package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/nats-io/nats.go"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/realtime"
	"github.com/runforyou-ai/cervi/internal/realtime/protocol"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// Path 是实时连接的升级路径。
const Path = "/api/realtime"

// flushTimeout 是等待 NATS 确认订阅生效的上限。
const flushTimeout = 5 * time.Second

// MemberBackend 解析成员登录令牌并读取同步探针值。
type MemberBackend interface {
	// AuthenticateMember 校验登录令牌并返回当前身份，令牌无效或账号不可用时返回登录会话错误。
	AuthenticateMember(ctx context.Context, token string) (*servermodels.Identity, error)
	// MemberSyncHeads 返回指定身份的同步探针值。
	MemberSyncHeads(ctx context.Context, identity *servermodels.Identity) (appservice.SyncHeads, error)
}

// Options 定义连接时限、发送队列与单帧上限。
type Options struct {
	AuthTimeout     time.Duration
	IdleTimeout     time.Duration
	MaxLifetime     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	QueueSize       int
	MaxFrameBytes   int
}

// DefaultOptions 返回首版连接参数：5 秒内认证、60 秒未收到帧即关闭、最长存活 1 小时。
func DefaultOptions(maxFrameBytes int) Options {
	return Options{
		AuthTimeout:     5 * time.Second,
		IdleTimeout:     60 * time.Second,
		MaxLifetime:     time.Hour,
		WriteTimeout:    10 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		QueueSize:       256,
		MaxFrameBytes:   maxFrameBytes,
	}
}

// Gateway 管理本节点的成员实时连接与受众订阅。
type Gateway struct {
	backend   MemberBackend
	namespace string
	options   Options

	mu          sync.Mutex
	nats        *nats.Conn
	closing     bool
	connections map[*connection]struct{}
	audiences   map[string]*audience
	running     sync.WaitGroup
}

// audience 是一个受众 Subject 的 NATS 订阅及本节点订阅该受众的连接。
type audience struct {
	subscription *nats.Subscription
	connections  map[*connection]struct{}
}

// New 创建使用指定 NATS 命名空间的成员实时网关。
func New(backend MemberBackend, namespace string, options Options) *Gateway {
	return &Gateway{
		backend:     backend,
		namespace:   namespace,
		options:     options,
		connections: map[*connection]struct{}{},
		audiences:   map[string]*audience{},
	}
}

// Start 使用指定 NATS 连接开始接收实时连接。
func (g *Gateway) Start(connection *nats.Conn) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nats = connection
	slog.Info("实时网关已启动", "namespace", g.namespace, "path", Path)
}

// Middleware 在 Wails 资源服务之前处理实时连接升级请求，其余请求交给下一个处理器。
func (g *Gateway) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != Path || !strings.EqualFold(request.Header.Get("Upgrade"), "websocket") {
			next.ServeHTTP(writer, request)
			return
		}
		g.serve(writer, request)
	})
}

// Shutdown 停止接收新连接，向现有连接发送下线提示并在时限内关闭。
func (g *Gateway) Shutdown() {
	g.mu.Lock()
	g.closing = true
	connections := make([]*connection, 0, len(g.connections))
	for current := range g.connections {
		connections = append(connections, current)
	}
	g.mu.Unlock()

	for _, current := range connections {
		current.close(false, websocket.StatusGoingAway, string(protocol.CloseServerGoingAway), protocol.ServerGoingAway{})
	}
	done := make(chan struct{})
	go func() {
		g.running.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(g.options.ShutdownTimeout):
		slog.Warn("实时连接未在时限内关闭，强制断开", "count", len(connections))
		for _, current := range connections {
			_ = current.socket.CloseNow()
		}
		<-done
	}
	slog.Info("实时网关已停止", "namespace", g.namespace, "closed", len(connections))
}

// serve 升级请求并运行连接直到关闭。
func (g *Gateway) serve(writer http.ResponseWriter, request *http.Request) {
	g.mu.Lock()
	unavailable := g.closing || g.nats == nil
	g.mu.Unlock()
	if unavailable {
		http.Error(writer, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}
	// Wails 资源服务的写入器延迟下发响应头，沿 Unwrap 取得底层写入器后再升级。
	for {
		unwrapper, ok := writer.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			break
		}
		writer = unwrapper.Unwrap()
	}
	socket, err := websocket.Accept(writer, request, nil)
	if err != nil {
		slog.Warn("实时连接升级失败", "error", err)
		return
	}
	socket.SetReadLimit(int64(g.options.MaxFrameBytes))
	current := newConnection(g, socket)

	g.mu.Lock()
	if g.closing {
		g.mu.Unlock()
		_ = socket.Close(websocket.StatusGoingAway, string(protocol.CloseServerGoingAway))
		return
	}
	g.connections[current] = struct{}{}
	g.running.Add(1)
	g.mu.Unlock()
	defer g.unregister(current)

	current.run(context.WithoutCancel(request.Context()))
}

// subscribe 让连接加入其用户受众，首个连接建立 NATS 订阅，并在 NATS 确认订阅生效后返回。
func (g *Gateway) subscribe(current *connection, identity *servermodels.Identity) error {
	subject := realtime.Subject(g.namespace, identity.Organization.ID, realtime.AudienceUser, identity.User.ID)
	g.mu.Lock()
	target := g.audiences[subject]
	if target == nil {
		subscription, err := g.nats.Subscribe(subject, func(message *nats.Msg) {
			g.deliver(subject, message.Data)
		})
		if err != nil {
			g.mu.Unlock()
			return err
		}
		target = &audience{subscription: subscription, connections: map[*connection]struct{}{}}
		g.audiences[subject] = target
	}
	target.connections[current] = struct{}{}
	current.subject = subject
	current.tokenSessionID = identity.Token.ID
	connection := g.nats
	g.mu.Unlock()

	// NATS 暂不可达时照常返回探针值，期间丢失的通知由客户端兜底探针恢复。
	if err := connection.FlushTimeout(flushTimeout); err != nil {
		slog.Warn("实时订阅确认失败", "connection_id", current.id, "user_id", identity.User.ID, "error", err)
	}
	return nil
}

// unregister 移除连接及其受众登记，受众不再有连接时取消 NATS 订阅。
func (g *Gateway) unregister(current *connection) {
	g.mu.Lock()
	delete(g.connections, current)
	if target := g.audiences[current.subject]; target != nil {
		delete(target.connections, current)
		if len(target.connections) == 0 {
			delete(g.audiences, current.subject)
			if err := target.subscription.Unsubscribe(); err != nil {
				slog.Warn("取消实时受众订阅失败", "subject", current.subject, "error", err)
			}
		}
	}
	g.mu.Unlock()
	g.running.Done()
}

// deliver 把受众通知转换为实时帧发给该受众的全部本节点连接，撤销控制关闭对应连接。
func (g *Gateway) deliver(subject string, data []byte) {
	var payload realtime.Payload
	if err := json.Unmarshal(data, &payload); err != nil {
		slog.Warn("解析实时通知失败", "subject", subject, "error", err)
		return
	}
	g.mu.Lock()
	var targets []*connection
	if target := g.audiences[subject]; target != nil {
		targets = make([]*connection, 0, len(target.connections))
		for current := range target.connections {
			targets = append(targets, current)
		}
	}
	g.mu.Unlock()

	var frame protocol.Frame
	switch payload.Kind {
	case realtime.KindConversationChanged:
		frame = protocol.ConversationChanged{ConversationID: payload.ConversationID, Version: payload.Version}
	case realtime.KindConversationRemoved:
		frame = protocol.ConversationRemoved{ConversationID: payload.ConversationID}
	case realtime.KindConversationStateChanged:
		frame = protocol.ConversationStateChanged{ConversationID: payload.ConversationID, Version: payload.Version}
	case realtime.KindIdentityProfileChanged:
		frame = protocol.IdentityProfileChanged{Version: payload.Version}
	case realtime.KindSessionLoggedOut:
		for _, current := range targets {
			if current.tokenSessionID == payload.TokenSessionID {
				current.revoke(protocol.SessionRevokedLogout)
			}
		}
		return
	case realtime.KindUserDisabled:
		for _, current := range targets {
			current.revoke(protocol.SessionRevokedUserDisabled)
		}
		return
	default:
		return
	}
	for _, current := range targets {
		current.send(frame)
	}
}
