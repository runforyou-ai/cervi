//go:build server

// Package gateway 在企业服务端内提供成员与网站访客的实时 SSE 事件流，按已认证身份订阅受众通知并转发为实时事件。
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/runforyou-ai/cervi/internal/appservice"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/realtime"
	"github.com/runforyou-ai/cervi/internal/realtime/protocol"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// Path 是成员实时事件流路径。
const Path = "/api/realtime"

// RunPath 是运行过程流路径前缀，其后是运行编号。
const RunPath = "/api/realtime/runs/"

// flushTimeout 是等待 NATS 确认订阅生效的上限。
const flushTimeout = 5 * time.Second

// VisitorBackend 解析网站访客的渠道身份。
type VisitorBackend interface {
	// AuthenticateVisitor 校验启用的网站渠道与访客身份并返回事件流受众，渠道停用或尚未建立身份时返回访客业务错误。
	AuthenticateVisitor(ctx context.Context, meta appservice.WebsiteVisitorMeta, channelID, externalID string) (appservice.WebsiteVisitorAudience, error)
	// VerifyCustomer 按渠道所属企业当前的客户身份密钥校验签名身份，失效时返回访客业务错误。
	VerifyCustomer(ctx context.Context, meta appservice.WebsiteVisitorMeta, channelID, token string) (appservice.WebsiteVisitorCustomer, error)
}

// MemberBackend 解析成员登录令牌并读取同步探针值。
type MemberBackend interface {
	// AuthenticateMember 校验请求携带的登录令牌并返回当前身份，令牌无效或账号不可用时返回登录会话错误。
	AuthenticateMember(ctx context.Context, meta appservice.RequestMeta) (*servermodels.Identity, error)
	// AuthenticateDevice 校验登录令牌与请求携带的本人未撤销设备并返回当前身份。
	AuthenticateDevice(ctx context.Context, meta appservice.RequestMeta) (*servermodels.Identity, error)
	// MemberSyncHeads 返回指定身份的同步探针值。
	MemberSyncHeads(ctx context.Context, identity *servermodels.Identity) (appservice.SyncHeads, error)
	// AuthorizeAgentRunStream 校验指定身份对运行所属会话的阅读资格，并返回运行所属会话编号。
	AuthorizeAgentRunStream(ctx context.Context, meta appservice.RequestMeta, identity *servermodels.Identity, runID string) (string, error)
	// SubscribeAgentRunStream 订阅本进程中该运行当前执行尝试的过程流，返回订阅时的快照与取消订阅函数；
	// 回调在运行流锁内串行执行，不得阻塞。运行不在本进程执行时返回 false，调用方按持久事实收敛。
	SubscribeAgentRunStream(runID string, onDelta func(agentruntime.StreamDelta), onEnd func()) (agentruntime.StreamSnapshot, func(), bool)
}

// Options 定义事件流心跳、时限与发送队列。
type Options struct {
	PingInterval    time.Duration
	MaxLifetime     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	QueueSize       int
	// RunSnapshotPartBytes 是运行过程流快照单个分片的文本预算。
	RunSnapshotPartBytes int
	// RunPendingTextBytes 是运行过程流待发增量合并后的文本上限，超过即按慢消费者结束该流；
	// 单个事件连同编码开销必须小于原生端读取单行事件的上限。
	RunPendingTextBytes int
}

// DefaultOptions 返回首版事件流参数：每 25 秒发送心跳、最长存活 1 小时。
func DefaultOptions() Options {
	return Options{
		PingInterval:         25 * time.Second,
		MaxLifetime:          time.Hour,
		WriteTimeout:         10 * time.Second,
		ShutdownTimeout:      5 * time.Second,
		QueueSize:            256,
		RunSnapshotPartBytes: 32 * 1024,
		RunPendingTextBytes:  256 * 1024,
	}
}

// memberFrameTypes 是成员事件流可下发的变更通知事件。
var memberFrameTypes = []protocol.Type{
	protocol.TypeServerHello, protocol.TypeConversationChanged, protocol.TypeConversationRemoved,
	protocol.TypeConversationStateChanged, protocol.TypeConversationTyping, protocol.TypeIdentityProfileChanged,
	protocol.TypePinOrderChanged, protocol.TypeServiceAttention,
}

// deviceFrameTypes 是携带设备身份的成员事件流额外可下发的事件。
var deviceFrameTypes = []protocol.Type{protocol.TypeDeviceWorkAdvanced}

// visitorFrameTypes 是网站访客事件流可下发的公开事件。
var visitorFrameTypes = []protocol.Type{protocol.TypeVisitorHello, protocol.TypeConversationChanged, protocol.TypeVisitorTyping}

// streamRoute 是一条已授权事件流的受众、撤销标识、可下发事件与授权到期时间。
type streamRoute struct {
	subjects       []string
	allowed        []protocol.Type
	tokenSessionID string
	// deviceID 是事件流携带的已认证设备编号，只有该设备的工作水位通知会下发。
	deviceID string
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
	streams      map[audienceStream]struct{}
	audiences    map[string]*audience
	running      sync.WaitGroup
	shutdownOnce sync.Once
}

// audienceStream 是加入受众订阅的事件流；登出与停用的撤销控制据此结束事件流。
type audienceStream interface {
	// tokenSession 返回事件流所属登录会话编号。
	tokenSession() string
	// audienceSubjects 返回事件流加入的受众 Subject。
	audienceSubjects() []string
	// revoke 因撤销控制清除未发送的事件并结束事件流。
	revoke(kind realtime.Kind)
	// shutdown 在网关下线时停止接收新事件，由写协程发送剩余事件后结束事件流。
	shutdown()
	// abort 取消请求处理，让阻塞中的写入立即超时。
	abort()
}

// audience 是一个受众 Subject 的 NATS 订阅及本节点订阅该受众的事件流。
type audience struct {
	subscription *nats.Subscription
	streams      map[audienceStream]struct{}
}

// New 创建使用指定 NATS 命名空间的实时网关，visitor 为 nil 时不提供访客事件流。
func New(backend MemberBackend, visitor VisitorBackend, namespace string, options Options) *Gateway {
	return &Gateway{
		backend:   backend,
		visitor:   visitor,
		namespace: namespace,
		options:   options,
		streams:   map[audienceStream]struct{}{},
		audiences: map[string]*audience{},
	}
}

// Start 使用指定 NATS 连接开始接收事件流请求。
func (g *Gateway) Start(connection *nats.Conn) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nats = connection
	slog.Info("实时网关已启动", "namespace", g.namespace, "path", Path)
}

// Middleware 在 Wails 资源服务之前处理成员事件流与运行过程流请求，其余请求交给下一个处理器。
func (g *Gateway) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			if request.URL.Path == Path {
				meta := appservice.RequestMeta{
					Token: bearerToken(request.Header.Get("Authorization")), Locale: appservice.Locale(request.Header.Get("Accept-Language")),
					DeviceID: strings.TrimSpace(request.Header.Get(appservice.DeviceHeader)),
				}
				g.stream(writer, request, meta, func(ctx context.Context) (streamRoute, error) {
					return g.memberRoute(ctx, meta)
				})
				return
			}
			if runID, ok := strings.CutPrefix(request.URL.Path, RunPath); ok && runID != "" && !strings.Contains(runID, "/") {
				g.serveRun(writer, request, runID)
				return
			}
		}
		next.ServeHTTP(writer, request)
	})
}

// ServeVisitor 处理已通过访客授权的网站访客事件流请求，访客元信息、channelID 与 externalID 由公开路由的访客授权得到。
func (g *Gateway) ServeVisitor(writer http.ResponseWriter, request *http.Request, meta appservice.WebsiteVisitorMeta, channelID, externalID string) {
	g.stream(writer, request, appservice.RequestMeta{Locale: meta.Locale}, func(ctx context.Context) (streamRoute, error) {
		return g.visitorRoute(ctx, meta, channelID, externalID)
	})
}

// Shutdown 停止接收新请求，结束现有事件流并在时限内等待其退出；重复调用等待首次调用完成。
func (g *Gateway) Shutdown() {
	g.shutdownOnce.Do(func() {
		g.mu.Lock()
		g.closing = true
		streams := make([]audienceStream, 0, len(g.streams))
		for current := range g.streams {
			streams = append(streams, current)
		}
		g.mu.Unlock()

		for _, current := range streams {
			current.shutdown()
		}
		done := make(chan struct{})
		go func() {
			g.running.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(g.options.ShutdownTimeout):
			slog.Warn("实时事件流未在时限内结束，强制断开", "count", len(streams))
			for _, current := range streams {
				current.abort()
			}
			<-done
		}
		slog.Info("实时网关已停止", "namespace", g.namespace, "closed", len(streams))
	})
}

// memberRoute 认证成员登录令牌，返回本人用户受众与本企业客服共享受众；携带设备编号时同时认证设备并下发该设备的工作水位。
func (g *Gateway) memberRoute(ctx context.Context, meta appservice.RequestMeta) (streamRoute, error) {
	authenticate := g.backend.AuthenticateMember
	allowed := memberFrameTypes
	if meta.DeviceID != "" {
		authenticate = g.backend.AuthenticateDevice
		allowed = append(slices.Clone(memberFrameTypes), deviceFrameTypes...)
	}
	identity, err := authenticate(ctx, meta)
	if err != nil {
		return streamRoute{}, err
	}
	organizationID := identity.Organization.ID
	attributes := []any{"organization_id", organizationID, "user_id", identity.User.ID}
	if meta.DeviceID != "" {
		attributes = append(attributes, "device_id", meta.DeviceID)
	}
	return streamRoute{
		subjects: []string{
			realtime.Subject(g.namespace, organizationID, realtime.AudienceUser, identity.User.ID),
			// 当前阶段所有成员均可阅读客户会话，成员连接都接收客服共享受众通知。
			realtime.Subject(g.namespace, organizationID, realtime.AudienceCustomerInbox, organizationID),
		},
		allowed:        allowed,
		tokenSessionID: identity.Token.ID,
		deviceID:       meta.DeviceID,
		// 事件流最长存活时间不晚于登录会话到期。
		expiresAt:  identity.Token.ExpiresAt,
		attributes: attributes,
		greet: func(ctx context.Context, connectionID string) (protocol.Frame, error) {
			// 订阅生效后再次校验登录会话，之后提交的登出或停用经受众通知送达。
			if _, err := authenticate(ctx, meta); err != nil {
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
	subjects := []string{
		realtime.Subject(g.namespace, target.OrganizationID, realtime.AudienceVisitorDirectory, target.ChannelIdentityID),
		// 渠道停用的撤销控制按渠道发送，该渠道全部访客事件流据此结束。
		realtime.Subject(g.namespace, target.OrganizationID, realtime.AudienceWebsiteChannel, target.ChannelID),
	}
	// 登录用户的事件流在签名身份过期时结束，并随客户身份密钥重新生成撤销。
	var expiresAt time.Time
	if meta.Customer != nil {
		subjects = append(subjects, realtime.Subject(g.namespace, target.OrganizationID, realtime.AudienceCustomerIdentity, target.OrganizationID))
		expiresAt = meta.Customer.ExpiresAt
	}
	return streamRoute{
		subjects:   subjects,
		allowed:    visitorFrameTypes,
		expiresAt:  expiresAt,
		attributes: []any{"organization_id", target.OrganizationID, "channel_id", target.ChannelID, "channel_identity_id", target.ChannelIdentityID},
		greet: func(ctx context.Context, connectionID string) (protocol.Frame, error) {
			// 订阅生效后再次校验渠道与访客身份，登录用户按当前密钥重新验签；之后提交的渠道停用与密钥重新生成经受众通知送达。
			if _, err := g.visitor.AuthenticateVisitor(ctx, meta, channelID, externalID); err != nil {
				return nil, err
			}
			if meta.Customer != nil {
				if _, err := g.visitor.VerifyCustomer(ctx, meta, channelID, meta.CustomerToken); err != nil {
					return nil, err
				}
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
	if !g.register(current) {
		writeUnavailable(writer, meta)
		return
	}
	defer g.unregister(current)

	if err := g.joinAudiences(ctx, current); err != nil {
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

// register 登记事件流，网关正在下线或 NATS 尚未就绪时返回 false。
func (g *Gateway) register(stream audienceStream) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closing || g.nats == nil {
		return false
	}
	g.streams[stream] = struct{}{}
	g.running.Add(1)
	return true
}

// joinAudiences 让事件流加入其受众，受众的首个事件流建立 NATS 订阅，并在 NATS 确认订阅生效后返回。
func (g *Gateway) joinAudiences(ctx context.Context, stream audienceStream) error {
	g.mu.Lock()
	for _, subject := range stream.audienceSubjects() {
		target := g.audiences[subject]
		if target == nil {
			subscription, err := g.nats.Subscribe(subject, func(message *nats.Msg) {
				g.deliver(subject, message.Data)
			})
			if err != nil {
				g.mu.Unlock()
				return err
			}
			target = &audience{subscription: subscription, streams: map[audienceStream]struct{}{}}
			g.audiences[subject] = target
		}
		target.streams[stream] = struct{}{}
	}
	connection := g.nats
	g.mu.Unlock()

	// NATS 暂不可达时照常继续，期间丢失的通知由客户端兜底探针恢复。
	flushCtx, cancel := context.WithTimeout(ctx, flushTimeout)
	defer cancel()
	if err := connection.FlushWithContext(flushCtx); err != nil {
		slog.Warn("实时订阅确认失败", "error", err)
	}
	return nil
}

// unregister 移除事件流及其受众登记，受众不再有事件流时取消 NATS 订阅。
func (g *Gateway) unregister(stream audienceStream) {
	g.mu.Lock()
	delete(g.streams, stream)
	for _, subject := range stream.audienceSubjects() {
		target := g.audiences[subject]
		if target == nil {
			continue
		}
		delete(target.streams, stream)
		if len(target.streams) == 0 {
			delete(g.audiences, subject)
			if err := target.subscription.Unsubscribe(); err != nil {
				slog.Warn("取消实时受众订阅失败", "subject", subject, "error", err)
			}
		}
	}
	g.mu.Unlock()
	g.running.Done()
}

// deliver 把受众通知转换为实时事件发给该受众的全部本节点事件流，撤销控制结束对应事件流。
func (g *Gateway) deliver(subject string, data []byte) {
	var payload realtime.Payload
	if err := json.Unmarshal(data, &payload); err != nil {
		slog.Warn("解析实时通知失败", "subject", subject, "error", err)
		return
	}
	g.mu.Lock()
	var targets []audienceStream
	if target := g.audiences[subject]; target != nil {
		targets = make([]audienceStream, 0, len(target.streams))
		for current := range target.streams {
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
	case realtime.KindConversationTyping:
		frame = protocol.ConversationTyping{ConversationID: payload.ConversationID, SenderSubjectID: payload.SenderSubjectID, Active: payload.Active}
	case realtime.KindVisitorTyping:
		frame = protocol.VisitorTyping{ConversationID: payload.ConversationID, Active: payload.Active}
	case realtime.KindIdentityProfileChanged:
		frame = protocol.IdentityProfileChanged{Version: payload.Version}
	case realtime.KindPinOrderChanged:
		frame = protocol.PinOrderChanged{Version: payload.Version}
	case realtime.KindServiceAttention:
		frame = protocol.ServiceAttention{ConversationID: payload.ConversationID, ServiceSessionID: payload.ServiceSessionID, Reason: payload.AttentionReason}
	case realtime.KindDeviceWorkAdvanced:
		frame = protocol.DeviceWorkAdvanced{DeviceID: payload.DeviceID, WorkSeq: payload.Version}
	case realtime.KindSessionLoggedOut:
		for _, current := range targets {
			if current.tokenSession() == payload.TokenSessionID {
				current.revoke(payload.Kind)
			}
		}
		return
	case realtime.KindUserDisabled, realtime.KindChannelDisabled, realtime.KindCustomerIdentityRevoked:
		for _, current := range targets {
			current.revoke(payload.Kind)
		}
		return
	default:
		return
	}
	// 变更通知只发给成员事件流；运行过程流在所属会话失权时结束，其余通知与它无关。
	for _, current := range targets {
		if member, ok := current.(*connection); ok {
			// 设备工作水位只发给携带该设备身份的事件流。
			if payload.Kind == realtime.KindDeviceWorkAdvanced && member.deviceID != payload.DeviceID {
				continue
			}
			member.send(frame)
			continue
		}
		if run, ok := current.(*runStream); ok && payload.Kind == realtime.KindConversationRemoved && run.conversationID == payload.ConversationID {
			run.revoke(payload.Kind)
		}
	}
}
