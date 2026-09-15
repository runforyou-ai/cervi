//go:build !server

package apiproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/clientsession"
	"github.com/runforyou-ai/cervi/internal/realtime/protocol"
)

// emittedEvent 是原生端投递给前端的一条事件。
type emittedEvent struct {
	name string
	data any
}

// TestRealtimeConnection 验证原生端按服务器地址路径拼接连接地址，发送认证与 Hello，并投递服务端帧与关闭原因。
func TestRealtimeConnection(t *testing.T) {
	received := make(chan []byte, 2)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/cervi/api/realtime" {
			http.NotFound(writer, request)
			return
		}
		socket, err := websocket.Accept(writer, request, nil)
		if err != nil {
			t.Error(err)
			return
		}
		for range 2 {
			_, data, err := socket.Read(request.Context())
			if err != nil {
				t.Error(err)
				return
			}
			received <- data
		}
		frame, _ := protocol.Encode(protocol.ConversationChanged{ConversationID: "conversation-1", Version: 9007199254740993})
		if err := socket.Write(request.Context(), websocket.MessageText, frame); err != nil {
			t.Error(err)
			return
		}
		<-release
		_ = socket.Close(websocket.StatusPolicyViolation, string(protocol.CloseSessionRevoked))
	}))
	t.Cleanup(server.Close)

	serverURL := server.URL + "/cervi"
	store := &memoryStore{serverURL: serverURL, credentialSet: true, credential: clientsession.Credential{
		ServerURL: serverURL, OrganizationID: "organization", UserID: "user", Token: "native-token", ExpiresAt: time.Now().Add(time.Hour),
	}}
	events := make(chan emittedEvent, 4)
	sessions, err := clientsession.NewManager(context.Background(), store)
	if err != nil {
		t.Fatal(err)
	}
	backend, err := NewBackend(store, sessions, func(name string, data any) { events <- emittedEvent{name, data} })
	if err != nil {
		t.Fatal(err)
	}

	connection, err := backend.ConnectRealtime(context.Background(), appservice.RequestMeta{Locale: "zh-CN"}, appservice.RealtimeConnectInput{AppVersion: "1.2.3"})
	if err != nil {
		t.Fatal(err)
	}
	// 服务端依次收到认证帧与 Hello 帧，令牌只在 Go 侧发送。
	for _, want := range []protocol.Frame{
		protocol.Authenticate{Token: "native-token"},
		protocol.ClientHello{ClientKind: protocol.ClientDesktop, AppVersion: "1.2.3"},
	} {
		select {
		case data := <-received:
			frame, err := protocol.DecodeClient(data)
			if err != nil || !reflect.DeepEqual(frame, want) {
				t.Fatalf("client frame = %#v (%v), want %#v", frame, err, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("等待客户端帧超时")
		}
	}

	frame, _ := protocol.Encode(protocol.ConversationChanged{ConversationID: "conversation-1", Version: 9007199254740993})
	expectEvent(t, events, emittedEvent{appservice.RealtimeFrameEventName, appservice.RealtimeFrameEvent{ConnectionID: connection.ConnectionID, Frame: string(frame)}})
	close(release)
	expectEvent(t, events, emittedEvent{appservice.RealtimeClosedEventName, appservice.RealtimeClosedEvent{
		ConnectionID: connection.ConnectionID, Code: int(websocket.StatusPolicyViolation), Reason: string(protocol.CloseSessionRevoked),
	}})
}

// TestRealtimeConnectionRequiresLogin 验证没有登录凭据时拒绝建立实时连接。
func TestRealtimeConnectionRequiresLogin(t *testing.T) {
	backend, err := newTestBackend(&memoryStore{serverURL: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = backend.ConnectRealtime(context.Background(), appservice.RequestMeta{Locale: "zh-CN"}, appservice.RealtimeConnectInput{})
	if state := appservice.SessionStateOf(err); state != appservice.SessionStateLogin {
		t.Fatalf("session state = %q (%v), want login", state, err)
	}
}

// expectEvent 在时限内读取下一条事件并与期望比较。
func expectEvent(t *testing.T, events <-chan emittedEvent, want emittedEvent) {
	t.Helper()
	select {
	case got := <-events:
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("event = %#v, want %#v", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("等待事件 %s 超时", want.name)
	}
}
