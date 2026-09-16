//go:build server

// Package gateway 在企业服务端内提供成员与网站访客的实时 SSE 事件流，按已认证身份订阅受众通知并转发为实时事件。
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/runforyou-ai/cervi/internal/appservice"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/realtime"
	"github.com/runforyou-ai/cervi/internal/realtime/protocol"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// Path 是成员实时事件流路径。
const Path = "/api/realtime"

// flushTimeout 是等待 NATS 确认订阅生效的上限。
const flushTimeout = 5 * time.Second

// VisitorBackend 解析网站访客的渠道身份。
type VisitorBackend interface {
	// AuthenticateVisitor 校验启用的网站渠道与访客身份并返回事件流受众，渠道停用或尚未建立身份时返回访客业务错误。
	AuthenticateVisitor(ctx context.Context, meta appservice.WebsiteVisitorMeta, channelID, externalID string) (appservice.WebsiteVisitorAudience, error)
}

// MemberBackend 解析成员登录令牌并读取同步探针值。
type MemberBackend interface {
	// AuthenticateMember 校验请求携带的登录令牌并返回当前身份，令牌无效或账号不可用时返回登录会话错误。
	AuthenticateMember(ctx context.Context, meta appservice.RequestMeta) (*servermodels.Identity, error)
	// MemberSyncHeads 返回指定身份的同步探针值。
	MemberSyncHeads(ctx context.Context, identity *servermodels.Identity) (appservice.SyncHeads, error)
}

// Options 定义事件流心跳、时限与发送队列。
type Options struct {
	PingInterval    time.Duration
	MaxLifetime     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	QueueSize       int
}

// DefaultOptions 返回首版事件流参数：每 25 秒发送心跳、最长存活 1 小时。
func DefaultOptions() Options {
	return Options{
		PingInterval:    25 * time.Second,
		MaxLifetime:     time.Hour,
		WriteTimeout:    10 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		QueueSize:       256,
	}
}

// memberFrameTypes 是成员事件流可下发的事件。
var memberFrameTypes = []protocol.Type{
	protocol.TypeServerHello, protocol.TypeConversationChanged, protocol.TypeConversationRemoved,
	protocol.TypeConversationStateChanged, protocol.TypeIdentityProfileChanged,
}

// visitorFrameTypes 是网站访客事件流可下发的公开事件。
var visitorFrameTypes = []protocol.Type{protocol.TypeVisitorHello, protocol.TypeConversationChanged}

// streamRoute 是一条已授权事件流的受众、撤销标识、可下发事件与授权到期时间。
type streamRoute struct {
	subjects       []string
	allowed        []protocol.Type
	tokenSessionID string
	// expiresAt 是事件流授权的绝对到期时间，零值表示只受最长存活时间约束。
	expiresAt  time.Time
	attributes []any
	// greet 在受众订阅生效后复核授权并返回首个事件。
	greet func(ctx context.Context, connectionID string) (protocol.Frame, error)
}

// Gateway 管理本节点的实时事件流与受众订阅。
type Gateway struct {
	backend   MemberBackend
	visitor   VisitorBackend
	namespace string
	options   Options

	mu           sync.Mutex
	nats         *nats.Conn
	closing      bool
	connections  map[*connection]struct{}
	audiences    map[string]*audience
	running      sync.WaitGroup
	shutdownOnce sync.Once
}

// audience 是一个受众 Subject 的 NATS 订阅及本节点订阅该受众的连接。
type audience struct {
	subscription *nats.Subscription
	connections  map[*connection]struct{}
}

// New 创建使用指定 NATS 命名空间的实时网关，visitor 为 nil 时不提供访客事件流。
func New(backend MemberBackend, visitor VisitorBackend, namespace string, options Options) *Gateway {
	return &Gateway{
		backend:     backend,
		visitor:     visitor,
		namespace:   namespace,
		options:     options,
		connections: map[*connection]struct{}{},
		audiences:   map[string]*audience{},
	}
}

// Start 使用指定 NATS 连接开始接收事件流请求。
func (g *Gateway) Start(connection *nats.Conn) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nats = connection
	slog.Info("实时网关已启动", "namespace", g.namespace, "path", Path)
}

// Middleware 在 Wails 资源服务之前处理实时事件流请求，其余请求交给下一个处理器。
func (g *Gateway) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != Path || request.Method != http.MethodGet {
			next.ServeHTTP(writer, request)
			return
		}
		meta := appservice.RequestMeta{Token: bearerToken(request.Header.Get("Authorization")), Locale: appservice.Locale(request.Header.Get("Accept-Language"))}
		g.stream(writer, request, meta, func(ctx context.Context) (streamRoute, error) {
			return g.memberRoute(ctx, meta)
		})
	})
}

// ServeVisitor 处理已通过访客授权的网站访客事件流请求，channelID 与 externalID 由公开路由的访客授权得到。
func (g *Gateway) ServeVisitor(writer http.ResponseWriter, request *http.Request, channelID, externalID string) {
	visitorMeta := appservice.WebsiteVisitorMeta{Locale: appservice.Locale(request.Header.Get("Accept-Language"))}
	g.stream(writer, request, appservice.RequestMeta{Locale: visitorMeta.Locale}, func(ctx context.Context) (streamRoute, error) {
		return g.visitorRoute(ctx, visitorMeta, channelID, externalID)
	})
}

// Shutdown 停止接收新请求，结束现有事件流并在时限内等待其退出；重复调用等待首次调用完成。
func (g *Gateway) Shutdown() {
	g.shutdownOnce.Do(func() {
		g.mu.Lock()
		g.closing = true
		connections := make([]*connection, 0, len(g.connections))
		for current := range g.connections {
			connections = append(connections, current)
		}
		g.mu.Unlock()

		for _, current := range connections {
			current.close(false)
		}
		done := make(chan struct{})
		go func() {
			g.running.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(g.options.ShutdownTimeout):
			slog.Warn("实时事件流未在时限内结束，强制断开", "count", len(connections))
			for _, current := range connections {
				current.abort()
			}
			<-done
		}
		slog.Info("实时网关已停止", "namespace", g.namespace, "closed", len(connections))
	})
}

// memberRoute 认证成员登录令牌，返回本人用户受众与本企业客服共享受众。
func (g *Gateway) memberRoute(ctx context.Context, meta appservice.RequestMeta) (streamRoute, error) {
	identity, err := g.backend.AuthenticateMember(ctx, meta)
	if err != nil {
		return streamRoute{}, err
	}
	organizationID := identity.Organization.ID
	return streamRoute{
		subjects: []string{
			realtime.Subject(g.namespace, organizationID, realtime.AudienceUser, identity.User.ID),
			// 当前阶段所有成员均可阅读客户会话，成员连接都接收客服共享受众通知。
			realtime.Subject(g.namespace, organizationID, realtime.AudienceCustomerInbox, organizationID),
		},
		allowed:        memberFrameTypes,
		tokenSessionID: identity.Token.ID,
		// 事件流最长存活时间不晚于登录会话到期。
		expiresAt:  identity.Token.ExpiresAt,
		attributes: []any{"organization_id", organizationID, "user_id", identity.User.ID},
		greet: func(ctx context.Context, connectionID string) (protocol.Frame, error) {
			// 订阅生效后再次校验登录会话，之后提交的登出或停用经受众通知送达。
			if _, err := g.backend.AuthenticateMember(ctx, meta); err != nil {
				return nil, err
			}
			heads, err := g.backend.MemberSyncHeads(ctx, identity)
			if err != nil {
				return nil, err
			}
			return protocol.ServerHello{ConnectionID: connectionID, SyncHeads: heads}, nil
		},
	}, nil
}

// visitorRoute 解析访客渠道身份，返回其访客目录受众与所在渠道的撤销受众。
func (g *Gateway) visitorRoute(ctx context.Context, meta appservice.WebsiteVisitorMeta, channelID, externalID string) (streamRoute, error) {
	if g.visitor == nil {
		return streamRoute{}, appservice.UnavailableError(appservice.RequestMeta{Locale: meta.Locale}, cervii18n.ErrorServerUnavailable, nil).WithStatus(http.StatusServiceUnavailable)
	}
	target, err := g.visitor.AuthenticateVisitor(ctx, meta, channelID, externalID)
	if err != nil {
		return streamRoute{}, err
	}
	return streamRoute{
		subjects: []string{
			realtime.Subject(g.namespace, target.OrganizationID, realtime.AudienceVisitorDirectory, target.ChannelIdentityID),
			// 渠道停用的撤销控制按渠道发送，该渠道全部访客事件流据此结束。
			realtime.Subject(g.namespace, target.OrganizationID, realtime.AudienceWebsiteChannel, target.ChannelID),
		},
		allowed:    visitorFrameTypes,
		attributes: []any{"organization_id", target.OrganizationID, "channel_id", target.ChannelID, "channel_identity_id", target.ChannelIdentityID},
		greet: func(ctx context.Context, connectionID string) (protocol.Frame, error) {
			// 订阅生效后再次校验渠道与访客身份，之后提交的渠道停用经受众通知送达。
			if _, err := g.visitor.AuthenticateVisitor(ctx, meta, channelID, externalID); err != nil {
				return nil, err
			}
			return protocol.VisitorHello{ConnectionID: connectionID}, nil
		},
	}, nil
}

// stream 按 authorize 得到的受众安装订阅并复核授权后输出事件流，直到事件流结束。
func (g *Gateway) stream(writer http.ResponseWriter, request *http.Request, meta appservice.RequestMeta, authorize func(context.Context) (streamRoute, error)) {
	route, err := authorize(request.Context())
	if err != nil {
		writeError(writer, meta, err)
		return
	}
	ctx, cancel := context.WithCancel(request.Context())
	defer cancel()
	current := newConnection(g, cancel, route)
	attributes := append([]any{"connection_id", current.id}, route.attributes...)
	g.mu.Lock()
	if g.closing || g.nats == nil {
		g.mu.Unlock()
		writeUnavailable(writer, meta)
		return
	}
	g.connections[current] = struct{}{}
	g.running.Add(1)
	g.mu.Unlock()
	defer g.unregister(current)

	if err := g.subscribe(ctx, current, route); err != nil {
		slog.Warn("实时受众订阅失败", append(attributes, "error", err)...)
		writeUnavailable(writer, meta)
		return
	}
	hello, err := route.greet(ctx, current.id)
	if err != nil {
		writeError(writer, meta, err)
		return
	}

	// 事件流是长响应：清除服务器读超时，写超时按每次写入设置；网关已开始下线时不输出事件流。
	controller := http.NewResponseController(writer)
	if err := controller.SetReadDeadline(time.Time{}); err != nil {
		slog.Warn("清除实时事件流读超时失败", "connection_id", current.id, "error", err)
		writeUnavailable(writer, meta)
		return
	}
	if !current.attach(controller) {
		writeUnavailable(writer, meta)
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("X-Accel-Buffering", "no")
	writer.WriteHeader(http.StatusOK)
	current.send(hello)

	// 存活时长在计时器启动时结算，握手耗时计入授权到期时间之内。
	lifetime := g.options.MaxLifetime
	if !route.expiresAt.IsZero() {
		lifetime = min(lifetime, time.Until(route.expiresAt))
	}
	expiry := time.AfterFunc(lifetime, func() {
		slog.Info("实时事件流到达最长存活时间", "connection_id", current.id)
		current.close(true)
	})
	defer expiry.Stop()
	slog.Info("实时事件流已就绪", append(attributes, "lifetime", lifetime)...)
	current.run(ctx, writer, controller)
	slog.Info("实时事件流已结束", attributes...)
}

// bearerToken 从 Authorization 头解析 Bearer 令牌，格式不符时返回空串。
func bearerToken(authorization string) string {
	scheme, token, found := strings.Cut(strings.TrimSpace(authorization), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// writeUnavailable 以业务错误体输出服务暂不可用。
func writeUnavailable(writer http.ResponseWriter, meta appservice.RequestMeta) {
	writeError(writer, meta, appservice.UnavailableError(meta, cervii18n.ErrorServerUnavailable, nil).WithStatus(http.StatusServiceUnavailable))
}

// writeError 按业务 HTTP 接口的错误体输出业务错误，其余错误输出服务暂不可用。
func writeError(writer http.ResponseWriter, meta appservice.RequestMeta, err error) {
	var applicationError *appservice.Error
	if !errors.As(err, &applicationError) {
		slog.Warn("实时事件流请求处理失败", "error", err)
		writeUnavailable(writer, meta)
		return
	}
	if applicationError.State != "" {
		slog.Warn("实时事件流认证失败", "state", applicationError.State)
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(applicationError.HTTPStatus())
	if err := json.NewEncoder(writer).Encode(struct {
		Error *appservice.Error `json:"error"`
	}{applicationError}); err != nil {
		slog.Warn("写入实时事件流错误响应失败", "error", err)
	}
}

// subscribe 让连接加入其受众，受众的首个连接建立 NATS 订阅，并在 NATS 确认订阅生效后返回。
func (g *Gateway) subscribe(ctx context.Context, current *connection, route streamRoute) error {
	g.mu.Lock()
	for _, subject := range route.subjects {
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
		current.subjects = append(current.subjects, subject)
	}
	connection := g.nats
	g.mu.Unlock()

	// NATS 暂不可达时照常输出事件流，期间丢失的通知由客户端兜底恢复。
	flushCtx, cancel := context.WithTimeout(ctx, flushTimeout)
	defer cancel()
	if err := connection.FlushWithContext(flushCtx); err != nil {
		slog.Warn("实时订阅确认失败", append(append([]any{"connection_id", current.id}, route.attributes...), "error", err)...)
	}
	return nil
}

// unregister 移除连接及其受众登记，受众不再有连接时取消 NATS 订阅。
func (g *Gateway) unregister(current *connection) {
	g.mu.Lock()
	delete(g.connections, current)
	for _, subject := range current.subjects {
		target := g.audiences[subject]
		delete(target.connections, current)
		if len(target.connections) == 0 {
			delete(g.audiences, subject)
			if err := target.subscription.Unsubscribe(); err != nil {
				slog.Warn("取消实时受众订阅失败", "subject", subject, "error", err)
			}
		}
	}
	g.mu.Unlock()
	g.running.Done()
}

// deliver 把受众通知转换为实时事件发给该受众的全部本节点连接，撤销控制结束对应事件流。
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
				current.revoke(payload.Kind)
			}
		}
		return
	case realtime.KindUserDisabled, realtime.KindChannelDisabled:
		for _, current := range targets {
			current.revoke(payload.Kind)
		}
		return
	default:
		return
	}
	for _, current := range targets {
		current.send(frame)
	}
}
