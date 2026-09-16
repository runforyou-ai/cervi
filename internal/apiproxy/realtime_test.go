//go:build !server

package apiproxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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

// TestRealtimeConnectionHeaderTimeout 验证服务端迟迟不返回响应头时建立事件流在时限内失败。
func TestRealtimeConnectionHeaderTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		select {
		case <-release:
		case <-request.Context().Done():
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	previous := realtimeConnectTimeout
	realtimeConnectTimeout = 200 * time.Millisecond
	t.Cleanup(func() { realtimeConnectTimeout = previous })

	backend, _ := newRealtimeTestBackend(t, server.URL)
	started := time.Now()
	_, err := backend.ConnectRealtime(context.Background(), appservice.RequestMeta{Locale: "zh-CN"})
	var applicationError *appservice.Error
	if !errors.As(err, &applicationError) || applicationError.Kind != appservice.ErrorKindUnavailable || time.Since(started) > 5*time.Second {
		t.Fatalf("err = %v, elapsed = %v", err, time.Since(started))
	}
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

// TestRealtimeDisconnectClosesRunStreams 验证成员事件流断开时一并关闭全部运行过程流。
func TestRealtimeDisconnectClosesRunStreams(t *testing.T) {
	frame, err := protocol.Encode(protocol.RunStreamEnded{RunID: "run-1"})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		if strings.HasPrefix(request.URL.Path, "/api/realtime/runs/") {
			_, _ = writer.Write(append(append([]byte("data: "), frame...), '\n', '\n'))
		}
		writer.(http.Flusher).Flush()
		<-request.Context().Done()
	}))
	t.Cleanup(server.Close)

	backend, events := newRealtimeTestBackend(t, server.URL)
	meta := appservice.RequestMeta{Locale: "zh-CN"}
	member, err := backend.ConnectRealtime(context.Background(), meta)
	if err != nil {
		t.Fatal(err)
	}
	first, err := backend.ConnectAgentRunStream(context.Background(), meta, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := backend.ConnectAgentRunStream(context.Background(), meta, "run-2")
	if err != nil {
		t.Fatal(err)
	}
	// 运行过程事件携带运行编号，订阅方据此区分并发的运行过程流。
	expectEvent(t, events, emittedEvent{appservice.RealtimeRunFrameEventName,
		appservice.RealtimeRunFrameEvent{ConnectionID: first.ConnectionID, RunID: "run-1", Frame: string(frame)}})
	expectEvent(t, events, emittedEvent{appservice.RealtimeRunFrameEventName,
		appservice.RealtimeRunFrameEvent{ConnectionID: second.ConnectionID, RunID: "run-2", Frame: string(frame)}})

	if err := backend.DisconnectRealtime(context.Background(), meta); err != nil {
		t.Fatal(err)
	}
	// 三条事件流都结束，顺序不固定。
	closed := map[string]bool{}
	for range 3 {
		select {
		case event := <-events:
			switch data := event.data.(type) {
			case appservice.RealtimeClosedEvent:
				closed[data.ConnectionID] = true
			case appservice.RealtimeRunClosedEvent:
				closed[data.ConnectionID] = true
			default:
				t.Fatalf("event = %#v", event)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("等待事件流结束超时，已结束 %v", closed)
		}
	}
	if !closed[member.ConnectionID] || !closed[first.ConnectionID] || !closed[second.ConnectionID] {
		t.Fatalf("closed = %v", closed)
	}
}
