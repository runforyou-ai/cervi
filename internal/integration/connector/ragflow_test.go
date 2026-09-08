package connector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

// TestRAGFlowProbe 验证只读探测的认证、分页和响应错误分类。
func TestRAGFlowProbe(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		stage  connectiontest.Stage
		kind   connectiontest.FailureKind
	}{
		{name: "empty datasets", body: `{"code":0,"data":[]}`},
		{name: "datasets", body: `{"code":0,"data":[{"id":"dataset-1","name":"产品文档"}]}`},
		{name: "authentication code", body: `{"code":109,"data":[]}`, stage: connectiontest.StageAuthenticate, kind: connectiontest.FailureUnauthorized},
		{name: "unauthorized code", body: `{"code":401,"message":"Invalid API key"}`, stage: connectiontest.StageAuthenticate, kind: connectiontest.FailureUnauthorized},
		{name: "permission code", body: `{"code":108}`, stage: connectiontest.StageAuthorize, kind: connectiontest.FailureForbidden},
		{name: "forbidden code", body: `{"code":403}`, stage: connectiontest.StageAuthorize, kind: connectiontest.FailureForbidden},
		{name: "business failure", body: `{"code":100,"data":[]}`, stage: connectiontest.StageCapability, kind: connectiontest.FailureUnavailable},
		{name: "http unauthorized", status: http.StatusUnauthorized, body: `{"code":401}`, stage: connectiontest.StageAuthenticate, kind: connectiontest.FailureUnauthorized},
		{name: "http forbidden", status: http.StatusForbidden, body: `{"code":403}`, stage: connectiontest.StageAuthorize, kind: connectiontest.FailureForbidden},
		{name: "http failure", status: http.StatusBadGateway, body: `{"code":0,"data":[]}`, stage: connectiontest.StageConnect, kind: connectiontest.FailureUnavailable},
		{name: "missing code", body: `{"data":[]}`, stage: connectiontest.StageCapability, kind: connectiontest.FailureProtocol},
		{name: "null code", body: `{"code":null,"data":[]}`, stage: connectiontest.StageCapability, kind: connectiontest.FailureProtocol},
		{name: "missing data", body: `{"code":0}`, stage: connectiontest.StageCapability, kind: connectiontest.FailureProtocol},
		{name: "null data", body: `{"code":0,"data":null}`, stage: connectiontest.StageCapability, kind: connectiontest.FailureProtocol},
		{name: "object data", body: `{"code":0,"data":{}}`, stage: connectiontest.StageCapability, kind: connectiontest.FailureProtocol},
		{name: "html response", body: `<html>RAGFlow</html>`, stage: connectiontest.StageCapability, kind: connectiontest.FailureProtocol},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodGet || request.URL.Path != "/instance/api/v1/datasets" {
					t.Errorf("unexpected request: %s %s", request.Method, request.URL.Path)
				}
				if query := request.URL.Query(); query.Get("page") != "1" || query.Get("page_size") != "1" {
					t.Errorf("unexpected query: %s", request.URL.RawQuery)
				}
				if request.Header.Get("Authorization") != "Bearer ragflow-test-key" || request.Header.Get("Accept") != "application/json" {
					t.Error("unexpected request headers")
				}
				writer.Header().Set("Content-Type", "application/json")
				if test.status != 0 {
					writer.WriteHeader(test.status)
				}
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			probe, err := NewRegistry(server.Client()).NewProbe(Config{
				Type:   domain.IntegrationConnectionTypeRAGFlow,
				APIURL: server.URL + "/instance/", APIKey: "ragflow-test-key",
			})
			if err != nil {
				t.Fatalf("create probe: %v", err)
			}
			err = probe.Run(context.Background())
			if test.kind == "" {
				if err != nil {
					t.Fatalf("run probe: %v", err)
				}
				return
			}
			stage, kind, classified := connectiontest.Details(err)
			if !classified || stage != test.stage || kind != test.kind {
				t.Fatalf("error = %v, want stage %q and kind %q", err, test.stage, test.kind)
			}
		})
	}
}
