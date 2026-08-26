//go:build !server

package apiproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/clientsession"
)

// TestBusinessSystemProxyUsesTypedEndpoints 验证原生端业务系统管理调用使用类型化接口和 Bearer Token。
func TestBusinessSystemProxyUsesTypedEndpoints(t *testing.T) {
	calls := 0
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(writer, "unexpected authorization", http.StatusBadRequest)
			return
		}
		calls++
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/integrations/business-systems":
			writeTestJSON(writer, http.StatusOK, appservice.BusinessSystemList{BusinessSystems: []appservice.BusinessSystem{{
				ID: "business-system-1", Name: "企业 ERP", URL: "https://erp.example.com", Enabled: true,
			}}})
		case request.Method == http.MethodGet && request.URL.Path == "/api/integrations/business-systems/business-system-1":
			writeTestJSON(writer, http.StatusOK, appservice.BusinessSystem{
				ID: "business-system-1", Name: "企业 ERP", URL: "https://erp.example.com", Enabled: true,
			})
		case request.Method == http.MethodPost && request.URL.Path == "/api/integrations/business-systems":
			var input appservice.BusinessSystemInput
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			writeTestJSON(writer, http.StatusCreated, appservice.BusinessSystem{
				ID: "business-system-1", Name: input.Name, URL: input.URL, Enabled: input.Enabled,
			})
		case request.Method == http.MethodPut && request.URL.Path == "/api/integrations/business-systems/business-system-1":
			var input appservice.BusinessSystemInput
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			writeTestJSON(writer, http.StatusOK, appservice.BusinessSystem{
				ID: "business-system-1", Name: input.Name, URL: input.URL, Enabled: input.Enabled,
			})
		case request.Method == http.MethodDelete && request.URL.Path == "/api/integrations/business-systems/business-system-1":
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer remote.Close()

	store := &memoryStore{
		serverURL: remote.URL,
		credential: clientsession.Credential{
			ServerURL: remote.URL, OrganizationID: "organization-1", UserID: "user-1",
			Token: "test-token", ExpiresAt: time.Now().Add(time.Hour),
		},
		credentialSet: true,
	}
	backend, err := newTestBackend(store)
	if err != nil {
		t.Fatal(err)
	}
	meta := appservice.RequestMeta{Locale: "zh-CN"}

	list, err := backend.ListBusinessSystems(context.Background(), meta)
	if err != nil || len(list.BusinessSystems) != 1 {
		t.Fatalf("list = %#v, error = %v", list, err)
	}
	detail, err := backend.GetBusinessSystem(context.Background(), meta, "business-system-1")
	if err != nil || detail.ID != "business-system-1" {
		t.Fatalf("detail = %#v, error = %v", detail, err)
	}
	input := appservice.BusinessSystemInput{Name: "企业 ERP", URL: "https://erp.example.com", Enabled: true}
	created, err := backend.CreateBusinessSystem(context.Background(), meta, input)
	if err != nil || created.Name != input.Name || !created.Enabled {
		t.Fatalf("created = %#v, error = %v", created, err)
	}
	input.URL = "https://erp.example.com/workbench"
	input.Enabled = false
	updated, err := backend.UpdateBusinessSystem(context.Background(), meta, "business-system-1", input)
	if err != nil || updated.URL != input.URL || updated.Enabled {
		t.Fatalf("updated = %#v, error = %v", updated, err)
	}
	if err := backend.DeleteBusinessSystem(context.Background(), meta, "business-system-1"); err != nil {
		t.Fatal(err)
	}
	if calls != 5 {
		t.Fatalf("calls = %d, want 5", calls)
	}
}
