package modelprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestOllamaDiscovererMapsCapabilities 验证 Ollama 已安装模型按能力映射用途和输入模态，上下文窗口只取 Modelfile 设定的 num_ctx。
func TestOllamaDiscovererMapsCapabilities(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/tags":
			_, _ = writer.Write([]byte(`{"models":[{"model":"qwen3-vl:8b"},{"model":"bge-m3:latest"},{"model":"broken:latest"}]}`))
		case "/api/show":
			var payload struct {
				Model string `json:"model"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			switch payload.Model {
			case "qwen3-vl:8b":
				_, _ = writer.Write([]byte(`{"capabilities":["completion","tools","vision"],"parameters":"stop \"<|im_end|>\"\nnum_ctx 32768\n","model_info":{"general.architecture":"qwen3vl","qwen3vl.context_length":262144}}`))
			case "bge-m3:latest":
				// 未设定 num_ctx 时只能得到模型支持的上限，上下文窗口留空。
				_, _ = writer.Write([]byte(`{"capabilities":["embedding"],"model_info":{"general.architecture":"bert","bert.context_length":8192}}`))
			default:
				writer.WriteHeader(http.StatusInternalServerError)
			}
		default:
			t.Errorf("unexpected path %s", request.URL.Path)
		}
	}))

	discoverer, err := NewRegistry(server.Client()).NewDiscoverer(Config{
		Brand: domain.AIProviderBrandOllama, APIURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	models, err := discoverer.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 3 {
		t.Fatalf("models = %#v", models)
	}
	vision := models[0]
	if vision.Type != domain.AIModelTypeChat || len(vision.InputModalities) != 2 ||
		vision.ContextWindow != 32_768 || vision.MaxOutputTokens != 32_768 {
		t.Fatalf("vision model = %#v", vision)
	}
	if models[1].Type != domain.AIModelTypeEmbedding || models[1].ContextWindow != 0 || models[1].MaxOutputTokens != 0 {
		t.Fatalf("embedding model = %#v", models[1])
	}
	// 读取模型详情失败时保留列表中的基础信息。
	if models[2].Identifier != "broken:latest" || models[2].Type != domain.AIModelTypeChat || models[2].ContextWindow != 0 {
		t.Fatalf("fallback model = %#v", models[2])
	}
}

// TestOpenAICompatibleDiscovererReadsModelList 验证兼容服务只提供模型标识，且无凭据时不携带鉴权头。
func TestOpenAICompatibleDiscovererReadsModelList(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/models" {
			t.Errorf("path = %s", request.URL.Path)
		}
		if authorization := request.Header.Get("Authorization"); authorization != "" {
			t.Errorf("authorization = %q, want empty", authorization)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"object":"list","data":[{"id":"Qwen/Qwen3-8B"},{"id":""}]}`))
	}))

	discoverer, err := NewRegistry(server.Client()).NewDiscoverer(Config{
		Brand: domain.AIProviderBrandOpenAICompatible, APIURL: server.URL + "/v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	models, err := discoverer.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].Identifier != "Qwen/Qwen3-8B" || models[0].Name != "Qwen/Qwen3-8B" {
		t.Fatalf("models = %#v", models)
	}
	if models[0].Type != domain.AIModelTypeChat || models[0].ContextWindow != 0 || models[0].MaxOutputTokens != 0 {
		t.Fatalf("model = %#v", models[0])
	}
}

// TestRegistryRejectsUnsupportedDiscovery 验证模型目录固定的品牌不提供模型发现。
func TestRegistryRejectsUnsupportedDiscovery(t *testing.T) {
	registry := NewRegistry(http.DefaultClient)
	if registry.SupportsDiscovery(domain.AIProviderBrandDeepSeek) {
		t.Fatal("DeepSeek should not support discovery")
	}
	if !registry.SupportsDiscovery(domain.AIProviderBrandOllama) {
		t.Fatal("Ollama should support discovery")
	}
	if _, err := registry.NewDiscoverer(Config{Brand: domain.AIProviderBrandDeepSeek}); err == nil {
		t.Fatal("NewDiscoverer() error = nil")
	}
}
