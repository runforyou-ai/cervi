package connector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

// TestDifyAppProbe 验证 Dify 应用密钥通过应用信息接口探测。
func TestDifyAppProbe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/info" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		if authorization := request.Header.Get("Authorization"); authorization != "Bearer app-key" {
			t.Fatalf("unexpected authorization header: %s", authorization)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"name":"客服应用","mode":"chat"}`))
	}))
	defer server.Close()

	probe, err := NewRegistry(server.Client()).NewProbe(Config{
		Type: domain.IntegrationConnectionTypeDify, APIURL: server.URL + "/v1", APIKey: "app-key",
	})
	if err != nil {
		t.Fatalf("create probe: %v", err)
	}
	if err := probe.Run(context.Background()); err != nil {
		t.Fatalf("run probe: %v", err)
	}
}

// TestDifyKnowledgeProbeFallback 验证 Dify 知识库密钥回退到知识库列表接口。
func TestDifyKnowledgeProbeFallback(t *testing.T) {
	paths := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.URL.Path)
		if authorization := request.Header.Get("Authorization"); authorization != "Bearer dataset-key" {
			t.Fatalf("unexpected authorization header: %s", authorization)
		}
		if request.URL.Path == "/v1/info" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		if request.URL.Path != "/v1/datasets" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	probe, err := NewRegistry(server.Client()).NewProbe(Config{
		Type: domain.IntegrationConnectionTypeDify, APIURL: server.URL + "/v1", APIKey: "dataset-key",
	})
	if err != nil {
		t.Fatalf("create probe: %v", err)
	}
	if err := probe.Run(context.Background()); err != nil {
		t.Fatalf("run probe: %v", err)
	}
	if len(paths) != 2 || paths[0] != "/v1/info" || paths[1] != "/v1/datasets" {
		t.Fatalf("unexpected request paths: %v", paths)
	}
}

// TestDifyKnowledgeBaseLister 验证 Dify 知识库列表按页读取并保留文档模式。
func TestDifyKnowledgeBaseLister(t *testing.T) {
	pages := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/datasets" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		if authorization := request.Header.Get("Authorization"); authorization != "Bearer dataset-key" {
			t.Fatalf("unexpected authorization header: %s", authorization)
		}
		if limit := request.URL.Query().Get("limit"); limit != "100" {
			t.Fatalf("unexpected limit: %s", limit)
		}
		page := request.URL.Query().Get("page")
		pages = append(pages, page)
		writer.Header().Set("Content-Type", "application/json")
		switch page {
		case "1":
			_, _ = writer.Write([]byte(`{"data":[{"id":"dataset-1","name":"产品文档","doc_form":"hierarchical_model"}],"has_more":true}`))
		case "2":
			_, _ = writer.Write([]byte(`{"data":[{"id":"dataset-2","name":"常见问题","doc_form":"qa_model"},{"id":"dataset-3","name":"空知识库","doc_form":null}],"has_more":false}`))
		default:
			t.Fatalf("unexpected page: %s", page)
		}
	}))
	defer server.Close()

	items, err := NewDifyKnowledgeBaseLister(server.Client()).List(context.Background(), DifyKnowledgeBaseConfig{
		APIURL: server.URL + "/v1", APIKey: "dataset-key",
	})
	if err != nil {
		t.Fatalf("list knowledge bases: %v", err)
	}
	if len(pages) != 2 || pages[0] != "1" || pages[1] != "2" {
		t.Fatalf("unexpected pages: %v", pages)
	}
	if len(items) != 3 || items[0].ID != "dataset-1" || items[0].DocForm != "hierarchical_model" ||
		items[1].ID != "dataset-2" || items[1].DocForm != "qa_model" ||
		items[2].ID != "dataset-3" || items[2].DocForm != "" {
		t.Fatalf("unexpected knowledge bases: %#v", items)
	}
}

// TestN8NProbe 验证 n8n 探测使用公开工作流接口和 API 密钥请求头。
func TestN8NProbe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/instance/api/v1/workflows" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		if limit := request.URL.Query().Get("limit"); limit != "1" {
			t.Fatalf("unexpected limit: %s", limit)
		}
		if apiKey := request.Header.Get("X-N8N-API-KEY"); apiKey != "n8n-key" {
			t.Fatalf("unexpected api key: %s", apiKey)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	probe, err := NewRegistry(server.Client()).NewProbe(Config{
		Type: domain.IntegrationConnectionTypeN8N, APIURL: server.URL + "/instance", APIKey: "n8n-key",
	})
	if err != nil {
		t.Fatalf("create probe: %v", err)
	}
	if err := probe.Run(context.Background()); err != nil {
		t.Fatalf("run probe: %v", err)
	}
}

// TestProbeDoesNotFollowRedirects 验证探测请求不会把凭据带到重定向地址。
func TestProbeDoesNotFollowRedirects(t *testing.T) {
	redirected := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected = true
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	defer origin.Close()

	probe, err := NewRegistry(NewHTTPClient()).NewProbe(Config{
		Type: domain.IntegrationConnectionTypeN8N, APIURL: origin.URL, APIKey: "n8n-key",
	})
	if err != nil {
		t.Fatalf("create probe: %v", err)
	}
	err = probe.Run(context.Background())
	_, kind, classified := connectiontest.Details(err)
	if !classified || kind != connectiontest.FailureProtocol {
		t.Fatalf("unexpected probe error: %v", err)
	}
	if redirected {
		t.Fatal("redirect target received the probe request")
	}
}
