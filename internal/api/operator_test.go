//go:build server

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/runforyou-ai/cervi/internal/appservice"
)

const testOperatorCredential = "operator-credential-operator-credential"

// newTestOperatorService 创建用于测试的运营接口适配器。
func newTestOperatorService() *OperatorService {
	return NewOperatorService(appservice.NewOperatorDirectBackend(appservice.OperatorDeployment{
		Mode:                appservice.DeploymentModeManaged,
		ManagedDomainSuffix: "cervi.runforyou.app",
	}, testOperatorCredential))
}

// TestOperatorDeploymentRequiresCredential 验证运营接口只接受配置的运营凭据。
func TestOperatorDeploymentRequiresCredential(t *testing.T) {
	service := newTestOperatorService()
	for name, authorization := range map[string]string{
		"缺少凭据":  "",
		"凭据不匹配": "Bearer member-token",
		"缺少前缀":  testOperatorCredential,
	} {
		request := httptest.NewRequest(http.MethodGet, "/deployment", nil)
		if authorization != "" {
			request.Header.Set("Authorization", authorization)
		}
		recorder := httptest.NewRecorder()
		service.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s 返回了 %d", name, recorder.Code)
		}
		var body operatorErrorBody
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Error.Code != appservice.OperatorErrorCodeInvalidCredential || body.Error.RequestID == "" || body.Error.Message == "" {
			t.Fatalf("%s 的错误体不完整: %#v", name, body.Error)
		}
	}
}

// TestOperatorDeploymentReturnsDeployment 验证凭据有效时返回部署形态与域名后缀。
func TestOperatorDeploymentReturnsDeployment(t *testing.T) {
	service := newTestOperatorService()
	request := httptest.NewRequest(http.MethodGet, "/deployment", nil)
	request.Header.Set("Authorization", "Bearer "+testOperatorCredential)
	request.Header.Set(requestIDHeader, "provisioning-request")
	recorder := httptest.NewRecorder()
	service.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("返回了 %d", recorder.Code)
	}
	var deployment appservice.OperatorDeployment
	if err := json.Unmarshal(recorder.Body.Bytes(), &deployment); err != nil {
		t.Fatal(err)
	}
	if deployment.Mode != appservice.DeploymentModeManaged || deployment.ManagedDomainSuffix != "cervi.runforyou.app" {
		t.Fatalf("部署信息不正确: %#v", deployment)
	}
}

// TestOperatorErrorCarriesRequestID 验证错误响应回传调用方提交的请求关联标识。
func TestOperatorErrorCarriesRequestID(t *testing.T) {
	service := newTestOperatorService()
	request := httptest.NewRequest(http.MethodGet, "/deployment", nil)
	request.Header.Set(requestIDHeader, "provisioning-request")
	recorder := httptest.NewRecorder()
	service.ServeHTTP(recorder, request)
	var body operatorErrorBody
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.RequestID != "provisioning-request" {
		t.Fatalf("请求标识不正确: %#v", body.Error)
	}
}

// TestOperatorRejectsUnknownPath 验证运营路由不提供未登记的路径。
func TestOperatorRejectsUnknownPath(t *testing.T) {
	service := newTestOperatorService()
	request := httptest.NewRequest(http.MethodGet, "/organizations", nil)
	request.Header.Set("Authorization", "Bearer "+testOperatorCredential)
	recorder := httptest.NewRecorder()
	service.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("返回了 %d", recorder.Code)
	}
}
