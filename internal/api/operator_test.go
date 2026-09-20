//go:build server

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testOperatorCredential = "operator-credential-operator-credential"

// newTestOperatorService 创建用于测试的运营接口适配器。
func newTestOperatorService() *OperatorService {
	return NewOperatorService(Deployment{
		DeploymentID:        "cervi-hosting-1",
		Mode:                "managed",
		ManagedDomainSuffix: "cervi.runforyou.app",
	}, testOperatorCredential)
}

// TestOperatorDeploymentRequiresCredential 验证运营接口只接受配置的运营凭据。
func TestOperatorDeploymentRequiresCredential(t *testing.T) {
	service := newTestOperatorService()
	for name, authorization := range map[string]string{
		"缺少凭据":  "",
		"凭据不匹配": "Bearer member-token",
		"缺少前缀":  testOperatorCredential[:10],
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
		if body.Error.Code != operatorCodeInvalidCredential || body.Error.RequestID == "" {
			t.Fatalf("%s 的错误体不完整: %#v", name, body.Error)
		}
	}
}

// TestOperatorDeploymentReturnsDeployment 验证凭据有效时返回部署标识与域名后缀，并回传请求标识。
func TestOperatorDeploymentReturnsDeployment(t *testing.T) {
	service := newTestOperatorService()
	request := httptest.NewRequest(http.MethodGet, "/deployment", nil)
	request.Header.Set("Authorization", "Bearer "+testOperatorCredential)
	recorder := httptest.NewRecorder()
	service.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("返回了 %d", recorder.Code)
	}
	var deployment Deployment
	if err := json.Unmarshal(recorder.Body.Bytes(), &deployment); err != nil {
		t.Fatal(err)
	}
	if deployment.DeploymentID != "cervi-hosting-1" || deployment.ManagedDomainSuffix != "cervi.runforyou.app" {
		t.Fatalf("部署信息不正确: %#v", deployment)
	}
}

// TestOperatorRejectsUnknownPath 验证运营路由不提供未登记的路径。
func TestOperatorRejectsUnknownPath(t *testing.T) {
	service := newTestOperatorService()
	request := httptest.NewRequest(http.MethodGet, "/organizations", strings.NewReader(""))
	request.Header.Set("Authorization", "Bearer "+testOperatorCredential)
	recorder := httptest.NewRecorder()
	service.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("返回了 %d", recorder.Code)
	}
}
