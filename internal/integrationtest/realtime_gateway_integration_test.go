//go:build server

package integrationtest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/coder/websocket"
	"github.com/nats-io/nats.go"
	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	"github.com/runforyou-ai/cervi/internal/realtime/gateway"
	"github.com/runforyou-ai/cervi/internal/realtime/protocol"
	"github.com/runforyou-ai/cervi/internal/servertest"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/tenant"
	"github.com/uptrace/bun"
)

// realtimeGatewayHarness 是连到独立 NATS 命名空间实时网关的测试服务。
type realtimeGatewayHarness struct {
	gateway   *gateway.Gateway
	backend   *appservice.DirectBackend
	namespace string
	url       string
	nats      *nats.Conn
	tenantCtx context.Context
}

// startRealtimeGateway 按测试企业的访问地址启动发布器、实时网关与 HTTP 服务，wrap 可替换网关使用的成员后端。
func startRealtimeGateway(t *testing.T, f navigationFixture, options gateway.Options, wrap func(gateway.MemberBackend) gateway.MemberBackend) *realtimeGatewayHarness {
	t.Helper()
	ctx := context.Background()
	var accessHost string
	if err := f.db.NewSelect().Table("organizations").Column("access_host").Where("id = ?", f.owner.Organization.ID).Scan(ctx, &accessHost); err != nil {
		t.Fatal(err)
	}
	config := servertest.NATSConfig(t, "test_gateway_"+strings.ReplaceAll(uuid.NewV7().String(), "-", ""))
	publisher := realtime.NewPublisher(config)
	if err := publisher.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = publisher.Stop() })

	backend := appservice.NewDirectBackend(f.db, nil, serverstorage.NewTenantResolver(f.db), nil, nil, nil, nil)
	var member gateway.MemberBackend = backend
	if wrap != nil {
		member = wrap(backend)
	}
	realtimeGateway := gateway.New(member, config.Namespace, options)
	realtimeGateway.Start(publisher.Connection())
	handler := realtimeGateway.Middleware(http.NotFoundHandler())
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		handler.ServeHTTP(writer, request.WithContext(tenant.WithAccessHost(request.Context(), accessHost)))
	}))
	t.Cleanup(server.Close)
	t.Cleanup(realtimeGateway.Shutdown)
	return &realtimeGatewayHarness{
		gateway: realtimeGateway, backend: backend, namespace: config.Namespace,
		url: "ws" + strings.TrimPrefix(server.URL, "http") + gateway.Path, nats: publisher.Connection(),
		tenantCtx: tenant.WithAccessHost(ctx, accessHost),
	}
}

// testGatewayOptions 返回默认网关参数并缩短下线等待时间。
func testGatewayOptions() gateway.Options {
	options := gateway.DefaultOptions(64 << 10)
	options.ShutdownTimeout = 2 * time.Second
	return options
}

// realtimeTestClient 是测试用的实时连接客户端。
type realtimeTestClient struct {
	t      *testing.T
	socket *websocket.Conn
}

// dial 建立一条尚未认证的实时连接。
func (h *realtimeGatewayHarness) dial(t *testing.T) *realtimeTestClient {
	t.Helper()
	socket, _, err := websocket.Dial(context.Background(), h.url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = socket.CloseNow() })
	return &realtimeTestClient{t: t, socket: socket}
}

// connect 建立连接并完成认证与 Hello，返回服务端 Hello。
func (h *realtimeGatewayHarness) connect(t *testing.T, token string) (*realtimeTestClient, protocol.ServerHello) {
	t.Helper()
	client := h.dial(t)
	client.send(protocol.Authenticate{Token: token})
	client.expect(protocol.Authenticated{})
	client.send(protocol.ClientHello{ClientKind: protocol.ClientWeb, AppVersion: "test"})
	hello, ok := client.next().(protocol.ServerHello)
	if !ok {
		t.Fatal("首个服务端帧不是 server_hello")
	}
	return client, hello
}

// expectAuthenticationFailed 验证令牌无法通过实时连接认证。
func (h *realtimeGatewayHarness) expectAuthenticationFailed(t *testing.T, token string) {
	t.Helper()
	client := h.dial(t)
	client.send(protocol.Authenticate{Token: token})
	client.expect(protocol.RealtimeError{Code: protocol.ErrorAuthenticationFailed})
	client.expectClosed(websocket.StatusPolicyViolation, string(protocol.ErrorAuthenticationFailed))
}

// send 编码并发送一帧客户端帧。
func (c *realtimeTestClient) send(frame protocol.Frame) {
	c.t.Helper()
	data, err := protocol.Encode(frame)
	if err != nil {
		c.t.Fatal(err)
	}
	c.sendText(string(data))
}

// sendText 发送原始文本帧。
func (c *realtimeTestClient) sendText(text string) {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.socket.Write(ctx, websocket.MessageText, []byte(text)); err != nil {
		c.t.Fatal(err)
	}
}

// next 在时限内读取并解码下一帧服务端帧。
func (c *realtimeTestClient) next() protocol.Frame {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, data, err := c.socket.Read(ctx)
	if err != nil {
		c.t.Fatalf("读取实时帧: %v", err)
	}
	frame, err := protocol.DecodeServer(data)
	if err != nil {
		c.t.Fatalf("解码实时帧 %s: %v", data, err)
	}
	return frame
}

// expect 读取下一帧并与期望比较。
func (c *realtimeTestClient) expect(want protocol.Frame) {
	c.t.Helper()
	if got := c.next(); !reflect.DeepEqual(got, want) {
		c.t.Fatalf("frame = %#v, want %#v", got, want)
	}
}

// expectClosed 在时限内读到服务端关闭帧，并校验关闭码与原因。
func (c *realtimeTestClient) expectClosed(status websocket.StatusCode, reason string) {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, data, err := c.socket.Read(ctx)
	var closeError websocket.CloseError
	if !errors.As(err, &closeError) || closeError.Code != status || closeError.Reason != reason {
		c.t.Fatalf("close = %v（帧 %s），want %d %s", err, data, status, reason)
	}
}

// loginToken 登录测试账号并返回新签发的令牌。
func loginToken(t *testing.T, db *bun.DB, organizationID, email string) string {
	t.Helper()
	login, err := authaction.NewLoginAction(db).Execute(context.Background(), authaction.LoginInput{OrganizationID: organizationID, Email: email, Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	return login.Token
}

// commitBeforeHeads 在读取同步探针前执行一次写入，模拟订阅安装与探针读取之间提交的变化。
type commitBeforeHeads struct {
	gateway.MemberBackend
	commit func()
}

// MemberSyncHeads 先执行写入再读取同步探针。
func (b commitBeforeHeads) MemberSyncHeads(ctx context.Context, identity *servermodels.Identity) (appservice.SyncHeads, error) {
	b.commit()
	return b.MemberBackend.MemberSyncHeads(ctx, identity)
}

// TestRealtimeGatewayDelivery 验证同一用户多条连接都收到通知，登出只关闭对应登录会话，停用关闭全部连接且无法重连。
func TestRealtimeGatewayDelivery(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	organizationID := f.owner.Organization.ID
	h := startRealtimeGateway(t, f, testGatewayOptions(), nil)
	tokenA := loginToken(t, f.db, organizationID, "member@navigation.test")
	tokenB := loginToken(t, f.db, organizationID, "member@navigation.test")
	clientA, helloA := h.connect(t, tokenA)
	clientB, _ := h.connect(t, tokenB)
	if heads, err := h.backend.MemberSyncHeads(ctx, f.member); err != nil || !reflect.DeepEqual(helloA.SyncHeads, heads) {
		t.Fatalf("hello heads = %+v, want %+v (%v)", helloA.SyncHeads, heads, err)
	}

	// 群消息通知同一用户的两条连接。
	f.send(t, f.owner, "实时网关", false)
	changed := protocol.ConversationChanged{ConversationID: f.groupID, Version: loadConversationVersion(t, f.db, f.groupID)}
	clientA.expect(changed)
	clientB.expect(changed)

	// 未知帧被忽略，连接照常应答心跳。
	clientA.sendText(`{"v":1,"type":"subscribe_run","data":{"runId":"run-1"}}`)
	clientA.send(protocol.Ping{})
	clientA.expect(protocol.Pong{})

	// 撤销事务回滚时不发布，连接保持可用。
	identityA, err := h.backend.AuthenticateMember(h.tenantCtx, tokenA)
	if err != nil {
		t.Fatal(err)
	}
	errRollback := errors.New("rollback")
	if err := realtime.RunInTx(ctx, f.db, func(ctx context.Context, _ bun.Tx) error {
		realtime.Notify(ctx, realtime.UserSessionLoggedOut(organizationID, f.member.User.ID, identityA.Token.ID))
		return errRollback
	}); !errors.Is(err, errRollback) {
		t.Fatalf("rollback err = %v", err)
	}
	clientA.send(protocol.Ping{})
	clientA.expect(protocol.Pong{})

	// 登出只关闭该登录会话的连接，另一登录会话继续收到通知，被登出的令牌无法重连。
	if err := authaction.NewLogoutAction(f.db).Execute(ctx, organizationID, tokenA); err != nil {
		t.Fatal(err)
	}
	clientA.expect(protocol.SessionRevoked{Reason: protocol.SessionRevokedLogout})
	clientA.expectClosed(websocket.StatusPolicyViolation, string(protocol.CloseSessionRevoked))
	f.send(t, f.owner, "登出之后", false)
	clientB.expect(protocol.ConversationChanged{ConversationID: f.groupID, Version: loadConversationVersion(t, f.db, f.groupID)})
	h.expectAuthenticationFailed(t, tokenA)

	// 停用账号关闭该用户全部连接；同一事务的资料通知可能先于撤销帧送达。
	if _, err := useraction.NewUpdateStatusAction(f.db).Execute(ctx, f.owner, f.member.User.ID, domain.UserStatusInactive); err != nil {
		t.Fatal(err)
	}
	frame := clientB.next()
	if _, profile := frame.(protocol.IdentityProfileChanged); profile {
		frame = clientB.next()
	}
	if want := (protocol.SessionRevoked{Reason: protocol.SessionRevokedUserDisabled}); frame != want {
		t.Fatalf("frame = %#v, want %#v", frame, want)
	}
	clientB.expectClosed(websocket.StatusPolicyViolation, string(protocol.CloseSessionRevoked))
	h.expectAuthenticationFailed(t, tokenB)
}

// TestRealtimeGatewayHelloAfterSubscription 验证订阅安装与探针读取之间提交的消息同时体现在 Hello 探针与后续通知中。
func TestRealtimeGatewayHelloAfterSubscription(t *testing.T) {
	f := newNavigationFixture(t)
	token := loginToken(t, f.db, f.owner.Organization.ID, "member@navigation.test")
	h := startRealtimeGateway(t, f, testGatewayOptions(), func(backend gateway.MemberBackend) gateway.MemberBackend {
		return commitBeforeHeads{MemberBackend: backend, commit: func() {
			_, err := newGroupSendAction(f.db).Execute(context.Background(), f.owner, conversationaction.GroupTextMessageInput{ConversationID: f.groupID, ClientMessageID: uuid.NewV7().String(), Body: "探针之前"})
			if err != nil {
				t.Error(err)
			}
		}}
	})
	client := h.dial(t)
	client.send(protocol.Authenticate{Token: token})
	client.expect(protocol.Authenticated{})
	client.send(protocol.ClientHello{ClientKind: protocol.ClientWeb, AppVersion: "test"})

	// 通知与 Hello 的先后顺序不固定，两者都必须送达。
	var hello *protocol.ServerHello
	var changed *protocol.ConversationChanged
	for hello == nil || changed == nil {
		switch frame := client.next().(type) {
		case protocol.ServerHello:
			hello = &frame
		case protocol.ConversationChanged:
			changed = &frame
		default:
			t.Fatalf("unexpected frame %#v", frame)
		}
	}
	if want := (protocol.ConversationChanged{ConversationID: f.groupID, Version: loadConversationVersion(t, f.db, f.groupID)}); *changed != want {
		t.Fatalf("changed = %#v, want %#v", *changed, want)
	}
	if heads, err := h.backend.MemberSyncHeads(context.Background(), f.member); err != nil || !reflect.DeepEqual(hello.SyncHeads, heads) {
		t.Fatalf("hello heads = %+v, want %+v (%v)", hello.SyncHeads, heads, err)
	}
}

// TestRealtimeGatewayConnectionLimits 验证认证时限、协议错误、空闲时限、最长存活时间与服务端下线。
func TestRealtimeGatewayConnectionLimits(t *testing.T) {
	f := newNavigationFixture(t)
	token := loginToken(t, f.db, f.owner.Organization.ID, "member@navigation.test")

	t.Run("认证超时", func(t *testing.T) {
		options := testGatewayOptions()
		options.AuthTimeout = 200 * time.Millisecond
		client := startRealtimeGateway(t, f, options, nil).dial(t)
		client.expect(protocol.RealtimeError{Code: protocol.ErrorAuthenticationTimeout})
		client.expectClosed(websocket.StatusPolicyViolation, string(protocol.ErrorAuthenticationTimeout))
	})
	t.Run("首帧不是认证", func(t *testing.T) {
		client := startRealtimeGateway(t, f, testGatewayOptions(), nil).dial(t)
		client.send(protocol.Ping{})
		client.expect(protocol.RealtimeError{Code: protocol.ErrorInvalidFrame})
		client.expectClosed(websocket.StatusPolicyViolation, string(protocol.ErrorInvalidFrame))
	})
	t.Run("协议主版本不支持", func(t *testing.T) {
		client := startRealtimeGateway(t, f, testGatewayOptions(), nil).dial(t)
		client.sendText(`{"v":2,"type":"authenticate","data":{"token":"cervi"}}`)
		client.expect(protocol.RealtimeError{Code: protocol.ErrorUnsupportedVersion})
		client.expectClosed(websocket.StatusPolicyViolation, string(protocol.ErrorUnsupportedVersion))
	})
	t.Run("认证后 Hello 前登出", func(t *testing.T) {
		loggedOut := loginToken(t, f.db, f.owner.Organization.ID, "member@navigation.test")
		client := startRealtimeGateway(t, f, testGatewayOptions(), nil).dial(t)
		client.send(protocol.Authenticate{Token: loggedOut})
		client.expect(protocol.Authenticated{})
		if err := authaction.NewLogoutAction(f.db).Execute(context.Background(), f.owner.Organization.ID, loggedOut); err != nil {
			t.Fatal(err)
		}
		client.send(protocol.ClientHello{ClientKind: protocol.ClientWeb, AppVersion: "test"})
		client.expect(protocol.RealtimeError{Code: protocol.ErrorAuthenticationFailed})
		client.expectClosed(websocket.StatusPolicyViolation, string(protocol.ErrorAuthenticationFailed))
	})
	t.Run("未知帧刷新空闲时限", func(t *testing.T) {
		options := testGatewayOptions()
		options.IdleTimeout = 500 * time.Millisecond
		client, _ := startRealtimeGateway(t, f, options, nil).connect(t, token)
		// 每 200 毫秒发送一个未知帧，持续超过空闲时限后连接仍应答 Ping。
		for range 5 {
			time.Sleep(200 * time.Millisecond)
			client.sendText(`{"v":1,"type":"subscribe_run","data":{}}`)
		}
		client.send(protocol.Ping{})
		client.expect(protocol.Pong{})
	})
	t.Run("空闲超时", func(t *testing.T) {
		options := testGatewayOptions()
		options.IdleTimeout = 500 * time.Millisecond
		client, _ := startRealtimeGateway(t, f, options, nil).connect(t, token)
		client.expectClosed(websocket.StatusNormalClosure, string(protocol.CloseIdleTimeout))
	})
	t.Run("最长存活时间", func(t *testing.T) {
		options := testGatewayOptions()
		options.MaxLifetime = time.Second
		h := startRealtimeGateway(t, f, options, nil)
		client, _ := h.connect(t, token)
		client.expectClosed(websocket.StatusNormalClosure, string(protocol.CloseSessionExpired))
		h.connect(t, token)
	})
	t.Run("服务端下线", func(t *testing.T) {
		h := startRealtimeGateway(t, f, testGatewayOptions(), nil)
		client, _ := h.connect(t, token)
		// 客户端持续读取时，下线提示与关闭握手在下线时限内完成。
		stopped := make(chan struct{})
		go func() {
			h.gateway.Shutdown()
			close(stopped)
		}()
		client.expect(protocol.ServerGoingAway{})
		client.expectClosed(websocket.StatusGoingAway, string(protocol.CloseServerGoingAway))
		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Fatal("客户端完成关闭握手后网关未及时停止")
		}
		_, response, err := websocket.Dial(context.Background(), h.url, nil)
		if err == nil || response == nil || response.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("下线后连接 err = %v, response = %v", err, response)
		}
	})
}

// TestRealtimeGatewaySlowConsumer 验证停止读取的连接在有界时间内被关闭，其他连接照常收到通知。
func TestRealtimeGatewaySlowConsumer(t *testing.T) {
	f := newNavigationFixture(t)
	ctx := context.Background()
	organizationID := f.owner.Organization.ID
	options := testGatewayOptions()
	options.QueueSize = 8
	options.WriteTimeout = 500 * time.Millisecond
	h := startRealtimeGateway(t, f, options, nil)
	slow, _ := h.connect(t, loginToken(t, f.db, organizationID, "member@navigation.test"))
	fast, _ := h.connect(t, loginToken(t, f.db, organizationID, "owner@navigation.test"))

	// 向慢连接所属用户受众连续发布互不合并的通知，慢连接期间不读取。
	subject := realtime.Subject(h.namespace, organizationID, realtime.AudienceUser, f.member.User.ID)
	for range 200000 {
		data, err := json.Marshal(realtime.Payload{Kind: realtime.KindConversationChanged, ConversationID: uuid.NewV7().String(), Version: 1})
		if err != nil {
			t.Fatal(err)
		}
		if err := h.nats.Publish(subject, data); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.nats.Flush(); err != nil {
		t.Fatal(err)
	}

	// 慢连接恢复读取后在时限内读到连接结束。
	deadline, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	for {
		if _, _, err := slow.socket.Read(deadline); err != nil {
			if deadline.Err() != nil {
				t.Fatalf("慢连接未在时限内关闭: %v", err)
			}
			break
		}
	}

	if err := realtime.RunInTx(ctx, f.db, func(ctx context.Context, _ bun.Tx) error {
		realtime.Notify(ctx, realtime.UserConversationChanged(organizationID, f.owner.User.ID, f.groupID, 99))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	fast.expect(protocol.ConversationChanged{ConversationID: f.groupID, Version: 99})
}
