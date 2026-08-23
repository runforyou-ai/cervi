//go:build server

package health

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestLiveness 验证存活探针输出版本且限制请求方法。
func TestLiveness(t *testing.T) {
	service := NewLiveness("1.2.3")
	recorder := httptest.NewRecorder()
	service.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"version":"1.2.3"`) {
		t.Fatalf("存活探针响应 = %d %q", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	service.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/healthz", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST 状态码 = %d", recorder.Code)
	}
}
