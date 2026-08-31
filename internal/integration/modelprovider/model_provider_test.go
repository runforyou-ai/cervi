package modelprovider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

// TestRegistryUsesProviderReadOnlyEndpoints 验证内置供应商通过只读模型列表接口测试连接。
func TestRegistryUsesProviderReadOnlyEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		brand    domain.AIProviderBrand
		basePath string
		wantPath string
		response string
	}{
		{name: "DeepSeek", brand: domain.AIProviderBrandDeepSeek, wantPath: "/models", response: `{"object":"list","data":[]}`},
		{name: "OpenAI", brand: domain.AIProviderBrandOpenAI, basePath: "/v1", wantPath: "/v1/models", response: `{"object":"list","data":[]}`},
		{name: "阿里云百炼", brand: domain.AIProviderBrandAlibaba, wantPath: "/api/v1/models", response: `{"success":true,"output":{"models":[]}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodGet || request.URL.Path != test.wantPath {
					t.Errorf("request = %s %s, want GET %s", request.Method, request.URL.Path, test.wantPath)
				}
				if request.Header.Get("Authorization") != "Bearer test-key" {
					t.Error("missing bearer authorization")
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(test.response))
			}))

			probe, err := NewRegistry(server.Client()).NewProbe(Config{
				Brand: test.brand, APIKey: "test-key", APIURL: server.URL + test.basePath,
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := probe.Run(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestProbeClassifiesAuthenticationFailure 验证供应商拒绝密钥时返回统一认证错误。
func TestProbeClassifiesAuthenticationFailure(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
	}))
	probe, err := NewRegistry(server.Client()).NewProbe(Config{
		Brand: domain.AIProviderBrandDeepSeek, APIKey: "invalid-key", APIURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = probe.Run(context.Background())
	stage, kind, ok := connectiontest.Details(err)
	if !ok || stage != connectiontest.StageAuthenticate || kind != connectiontest.FailureUnauthorized {
		t.Fatalf("stage = %q, kind = %q, ok = %v", stage, kind, ok)
	}
}

// TestAlibabaModelsURLAcceptsCompatibleBaseURL 验证百炼兼容模式地址会切换到原生模型列表路径。
func TestAlibabaModelsURLAcceptsCompatibleBaseURL(t *testing.T) {
	got, err := alibabaModelsURL("https://dashscope.aliyuncs.com/compatible-mode/v1")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://dashscope.aliyuncs.com/api/v1/models"
	if got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
}
