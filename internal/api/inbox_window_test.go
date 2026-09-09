//go:build server

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/runforyou-ai/cervi/internal/appservice"
)

type inboxWindowBackend struct {
	appservice.Backend
	contextInput appservice.InboxContextInput
	windowInput  appservice.InboxWindowInput
}

// GetInboxContext 记录锚点请求并返回无权锚点的原邻域。
func (b *inboxWindowBackend) GetInboxContext(_ context.Context, _ appservice.RequestMeta, input appservice.InboxContextInput) (appservice.InboxContext, error) {
	b.contextInput = input
	return appservice.InboxContext{Anchor: appservice.InboxConversationResult{ID: input.AnchorID, Availability: appservice.InboxConversationUnavailable}, Window: appservice.InboxWindow{Conversations: []appservice.InboxConversation{}, StartCursor: "first", EndCursor: "last", HasBefore: true, HasAfter: true}}, nil
}

// ReadInboxWindow 记录双向边界并返回完整区间响应。
func (b *inboxWindowBackend) ReadInboxWindow(_ context.Context, _ appservice.RequestMeta, input appservice.InboxWindowInput) (appservice.InboxWindow, error) {
	b.windowInput = input
	return appservice.InboxWindow{Conversations: []appservice.InboxConversation{}, StartCursor: input.StartCursor, EndCursor: input.EndCursor, HasBefore: true}, nil
}

// TestInboxWindowHTTP 验证只读 POST 请求中的嵌套筛选、锚点和双向范围元数据。
func TestInboxWindowHTTP(t *testing.T) {
	backend := &inboxWindowBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()
	filter := appservice.InboxQuery{Scope: appservice.InboxScopeCustomer, CustomerView: appservice.CustomerInboxViewCoworkers, AssigneeIdentityID: "peer"}
	contextInput := appservice.InboxContextInput{Query: filter, AnchorID: "anchor", AnchorCursor: "original", BeforeLimit: 7, AfterLimit: 9}
	response := doJSON(t, http.MethodPost, server.URL+"/inbox/context/query", contextInput, "token")
	var located appservice.InboxContext
	err := json.NewDecoder(response.Body).Decode(&located)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || err != nil || backend.contextInput != contextInput || located.Anchor.Conversation != nil || located.Anchor.Availability != appservice.InboxConversationUnavailable || !located.Window.HasBefore || !located.Window.HasAfter || located.Window.StartCursor != "first" || located.Window.EndCursor != "last" || located.Window.Conversations == nil {
		t.Fatalf("context=%+v request=%+v err=%v", located, backend.contextInput, err)
	}
	windowInput := appservice.InboxWindowInput{Query: filter, StartCursor: "start", EndCursor: "end"}
	response = doJSON(t, http.MethodPost, server.URL+"/inbox/window/query", windowInput, "token")
	var window appservice.InboxWindow
	err = json.NewDecoder(response.Body).Decode(&window)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || err != nil || backend.windowInput != windowInput || window.StartCursor != "start" || window.EndCursor != "end" || !window.HasBefore || window.HasAfter || window.Conversations == nil {
		t.Fatalf("window=%+v request=%+v err=%v", window, backend.windowInput, err)
	}
}
