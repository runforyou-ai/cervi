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

type inboxPageBackend struct {
	appservice.Backend
	input appservice.LoadInboxInput
}

// LoadInbox 记录适配后的分页输入并返回续页元数据。
func (b *inboxPageBackend) LoadInbox(_ context.Context, _ appservice.RequestMeta, input appservice.LoadInboxInput) (appservice.Inbox, error) {
	b.input = input
	return appservice.Inbox{Conversations: []appservice.InboxConversation{}, StartCursor: "first", EndCursor: "last", HasBefore: true, HasMore: true, NextCursor: "next-page", UnreadCount: 80, AttentionUnreadCount: 70}, nil
}

// TestInboxPaginationHTTP 验证 GET 参数、默认页大小、非法页大小与分页响应。
func TestInboxPaginationHTTP(t *testing.T) {
	backend := &inboxPageBackend{}
	server := httptest.NewServer(NewService(appservice.New(backend)))
	defer server.Close()
	for _, test := range []struct {
		query  string
		limit  int
		cursor string
		before string
	}{
		{"", 50, "", ""},
		{"?scope=customer&customerView=coworkers&assigneeIdentityId=peer&limit=7&cursor=original-boundary", 7, "original-boundary", ""},
		{"?scope=internal&beforeCursor=previous-boundary&limit=8", 8, "", "previous-boundary"},
	} {
		response := doJSON(t, http.MethodGet, server.URL+"/inbox"+test.query, nil, "token")
		var page appservice.Inbox
		err := json.NewDecoder(response.Body).Decode(&page)
		response.Body.Close()
		if response.StatusCode != http.StatusOK || err != nil || backend.input.Limit != test.limit || backend.input.Cursor != test.cursor || backend.input.BeforeCursor != test.before || page.StartCursor != "first" || page.EndCursor != "last" || !page.HasBefore || !page.HasMore || page.NextCursor != "next-page" || page.UnreadCount != 80 || page.AttentionUnreadCount != 70 {
			t.Fatalf("input=%+v response=%+v err=%v", backend.input, page, err)
		}
		if test.cursor != "" && (backend.input.Scope != appservice.InboxScopeCustomer || backend.input.CustomerView != appservice.CustomerInboxViewCoworkers || backend.input.AssigneeIdentityID != "peer") {
			t.Fatalf("filters lost=%+v", backend.input)
		}
	}
	for _, value := range []string{"0", "-1", "abc"} {
		response := doJSON(t, http.MethodGet, server.URL+"/inbox?limit="+value, nil, "token")
		response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("invalid limit %s status=%d", value, response.StatusCode)
		}
	}
}
