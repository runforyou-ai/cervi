//go:build !server

package apiproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/clientsession"
	"github.com/runforyou-ai/cervi/internal/realtime/protocol"
)

// emittedEvent 是原生端投递给前端的一条事件。
type emittedEvent struct {
	name string
	data any
}

// newRealtimeTestBackend 创建持有登录凭据、连到指定服务器地址并记录投递事件的原生端后端。
func newRealtimeTestBackend(t *testing.T, serverURL string) (*Backend, <-chan emittedEvent) {
	t.Helper()
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
	return backend, events
}

// TestRealtimeConnection 验证原生端按服务器地址路径拼接事件流地址并携带凭据，投递服务端事件与事件流结束。
func TestRealtimeConnection(t *testing.T) {
	frame, err := protocol.Encode(protocol.ConversationChanged{ConversationID: "conversation-1", Version: 9007199254740993})
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/cervi/api/realtime" {
			http.NotFound(writer, request)
			return
		}
		if request.Header.Get("Authorization") != "Bearer native-token" || request.Header.Get("Accept-Language") != "zh-CN" {
			t.Errorf("headers = %v", request.Header)
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write(append(append([]byte("data: "), frame...), '\n', '\n'))
		writer.(http.Flusher).Flush()
		select {
		case <-release:
		case <-request.Context().Done():
		}
	}))
	t.Cleanup(server.Close)

	backend, events := newRealtimeTestBackend(t, server.URL+"/cervi")
	connection, err := backend.ConnectRealtime(context.Background(), appservice.RequestMeta{Locale: "zh-CN"})
	if err != nil {
		t.Fatal(err)
	}
	expectEvent(t, events, emittedEvent{appservice.RealtimeFrameEventName, appservice.RealtimeFrameEvent{ConnectionID: connection.ConnectionID, Frame: string(frame)}})
	close(release)
	expectEvent(t, events, emittedEvent{appservice.RealtimeClosedEventName, appservice.RealtimeClosedEvent{ConnectionID: connection.ConnectionID}})
}

// TestRealtimeConnectionRejectedCredential 验证服务端拒绝登录凭据时返回登录会话错误并清除本地凭据。
func TestRealtimeConnectionRejectedCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"error":{"state":"login","message":"请重新登录"}}`))
	}))
	t.Cleanup(server.Close)

	backend, _ := newRealtimeTestBackend(t, server.URL)
	_, err := backend.ConnectRealtime(context.Background(), appservice.RequestMeta{Locale: "zh-CN"})
	if state := appservice.SessionStateOf(err); state != appservice.SessionStateLogin {
		t.Fatalf("session state = %q (%v), want login", state, err)
	}
	if _, ok := backend.sessions.Current(context.Background(), server.URL); ok {
		t.Fatal("登录凭据被拒绝后仍保留本地凭据")
	}
}

// TestRealtimeConnectionRequiresLogin 验证没有登录凭据时拒绝建立实时事件流。
func TestRealtimeConnectionRequiresLogin(t *testing.T) {
	backend, err := newTestBackend(&memoryStore{serverURL: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = backend.ConnectRealtime(context.Background(), appservice.RequestMeta{Locale: "zh-CN"})
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
