//go:build server

package setting

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
)

const testS3OrganizationID = "00000000-0000-0000-0000-000000000001"

// s3ProbeServerState 记录端到端对象存储探针请求。
type s3ProbeServerState struct {
	mu            sync.Mutex
	preflightCORS bool
	putCORS       bool
	putStatus     int
	publicStatus  int
	deleteStatus  int
	storageKey    string
	payload       []byte
	deleted       bool
}

// TestS3SettingActionExecute 验证对象存储测试覆盖签名访问、CORS、上传、公开读取和清理。
func TestS3SettingActionExecute(t *testing.T) {
	server, state := newS3ProbeServer(t, true, true, http.StatusOK)
	defer server.Close()

	action := NewTestS3SettingAction(connectiontest.NewRunner(time.Second))
	if err := action.Execute(context.Background(), testS3OrganizationID, testS3Setting(server.URL)); err != nil {
		t.Fatal(err)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !strings.HasPrefix(state.storageKey, "organizations/"+testS3OrganizationID+"/files/") || !strings.HasSuffix(state.storageKey, ".txt") ||
		!strings.HasPrefix(string(state.payload), "cervi-object-storage-probe:") || !state.deleted {
		t.Fatalf("probe state = %#v", state)
	}
}

// TestS3SettingActionRejectsMissingCORS 验证预签名上传未允许跨域时测试失败且清理探针。
func TestS3SettingActionRejectsMissingCORS(t *testing.T) {
	server, state := newS3ProbeServer(t, false, true, http.StatusOK)
	defer server.Close()

	action := NewTestS3SettingAction(connectiontest.NewRunner(time.Second))
	err := action.Execute(context.Background(), testS3OrganizationID, testS3Setting(server.URL))
	assertS3TestFailure(t, err, S3TestFailureCORS)
	stage, kind, ok := connectiontest.Details(err)
	if !ok || stage != connectiontest.StageCapability || kind != connectiontest.FailureProtocol {
		t.Fatalf("stage = %q, kind = %q, ok = %v", stage, kind, ok)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.deleted {
		t.Fatal("probe object was not cleaned after CORS failure")
	}
}

// TestS3SettingActionRejectsMissingPutCORS 验证实际上传响应未允许跨域时测试失败。
func TestS3SettingActionRejectsMissingPutCORS(t *testing.T) {
	server, state := newS3ProbeServer(t, true, false, http.StatusOK)
	defer server.Close()

	action := NewTestS3SettingAction(connectiontest.NewRunner(time.Second))
	err := action.Execute(context.Background(), testS3OrganizationID, testS3Setting(server.URL))
	assertS3TestFailure(t, err, S3TestFailureCORS)
	stage, kind, ok := connectiontest.Details(err)
	if !ok || stage != connectiontest.StageCapability || kind != connectiontest.FailureProtocol {
		t.Fatalf("stage = %q, kind = %q, ok = %v", stage, kind, ok)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.payload) == 0 || !state.deleted {
		t.Fatalf("probe state = %#v", state)
	}
}

// TestS3SettingActionRejectsUpload 验证测试文件写入失败时返回上传阶段错误。
func TestS3SettingActionRejectsUpload(t *testing.T) {
	server, state := newS3ProbeServer(t, true, true, http.StatusOK)
	defer server.Close()
	state.putStatus = http.StatusForbidden

	action := NewTestS3SettingAction(connectiontest.NewRunner(time.Second))
	err := action.Execute(context.Background(), testS3OrganizationID, testS3Setting(server.URL))
	assertS3TestFailure(t, err, S3TestFailureUpload)
	stage, kind, ok := connectiontest.Details(err)
	if !ok || stage != connectiontest.StageAuthorize || kind != connectiontest.FailureForbidden {
		t.Fatalf("stage = %q, kind = %q, ok = %v", stage, kind, ok)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.deleted {
		t.Fatal("probe object was not cleaned after upload failure")
	}
}

// TestS3SettingActionRejectsPrivatePublicURL 验证匿名公开读取被拒绝时测试失败且清理已上传探针。
func TestS3SettingActionRejectsPrivatePublicURL(t *testing.T) {
	server, state := newS3ProbeServer(t, true, true, http.StatusForbidden)
	defer server.Close()

	action := NewTestS3SettingAction(connectiontest.NewRunner(time.Second))
	err := action.Execute(context.Background(), testS3OrganizationID, testS3Setting(server.URL))
	assertS3TestFailure(t, err, S3TestFailurePublicAccess)
	stage, kind, ok := connectiontest.Details(err)
	if !ok || stage != connectiontest.StageAuthorize || kind != connectiontest.FailureForbidden {
		t.Fatalf("stage = %q, kind = %q, ok = %v", stage, kind, ok)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.payload) == 0 || !state.deleted {
		t.Fatalf("probe state = %#v", state)
	}
}

// TestS3SettingActionRejectsCleanup 验证测试文件删除失败时返回清理阶段错误。
func TestS3SettingActionRejectsCleanup(t *testing.T) {
	server, state := newS3ProbeServer(t, true, true, http.StatusOK)
	defer server.Close()
	state.deleteStatus = http.StatusForbidden

	action := NewTestS3SettingAction(connectiontest.NewRunner(time.Second))
	err := action.Execute(context.Background(), testS3OrganizationID, testS3Setting(server.URL))
	assertS3TestFailure(t, err, S3TestFailureCleanup)
	stage, kind, ok := connectiontest.Details(err)
	if !ok || stage != connectiontest.StageAuthorize || kind != connectiontest.FailureForbidden {
		t.Fatalf("stage = %q, kind = %q, ok = %v", stage, kind, ok)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.deleted {
		t.Fatal("probe object was marked deleted after cleanup failure")
	}
}

// TestS3SettingActionReturnsConnectionError 验证存储桶拒绝签名访问时返回稳定错误。
func TestS3SettingActionReturnsConnectionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	action := NewTestS3SettingAction(connectiontest.NewRunner(time.Second))
	err := action.Execute(context.Background(), testS3OrganizationID, testS3Setting(server.URL))
	assertS3TestFailure(t, err, S3TestFailureBucketAccess)
	stage, kind, ok := connectiontest.Details(err)
	if !ok || stage != connectiontest.StageAuthorize || kind != connectiontest.FailureForbidden {
		t.Fatalf("stage = %q, kind = %q, ok = %v", stage, kind, ok)
	}
}

// assertS3TestFailure 验证对象存储探针保留了具体失败能力。
func assertS3TestFailure(t *testing.T, err error, want S3TestFailure) {
	t.Helper()
	got, ok := S3TestFailureOf(err)
	if !ok || got != want {
		t.Fatalf("S3 test failure = %q, ok = %v, want %q", got, ok, want)
	}
}

// newS3ProbeServer 创建同时模拟 S3 API 和公开对象地址的测试服务。
func newS3ProbeServer(t *testing.T, preflightCORS, putCORS bool, publicStatus int) (*httptest.Server, *s3ProbeServerState) {
	t.Helper()
	state := &s3ProbeServerState{
		preflightCORS: preflightCORS,
		putCORS:       putCORS,
		putStatus:     http.StatusOK,
		publicStatus:  publicStatus,
		deleteStatus:  http.StatusNoContent,
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		state.serve(t, writer, request)
	}))
	return server, state
}

// serve 处理测试探针使用的最小 S3 和公开读取请求。
func (s *s3ProbeServerState) serve(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	t.Helper()
	switch {
	case request.Method == http.MethodHead && request.URL.Path == "/cervi":
		if request.Header.Get("Authorization") == "" {
			t.Error("missing signed HeadBucket authorization")
		}
		writer.WriteHeader(http.StatusOK)
	case request.Method == http.MethodOptions && strings.HasPrefix(request.URL.Path, "/cervi/organizations/"):
		if request.Header.Get("Origin") != s3ProbeOrigin || request.Header.Get("Access-Control-Request-Method") != http.MethodPut {
			t.Errorf("preflight headers = %#v", request.Header)
		}
		requestedHeaders := strings.ToLower(request.Header.Get("Access-Control-Request-Headers"))
		if !strings.Contains(requestedHeaders, "cache-control") || !strings.Contains(requestedHeaders, "content-type") {
			t.Errorf("preflight requested headers = %q", requestedHeaders)
		}
		if s.preflightCORS {
			writer.Header().Set("Access-Control-Allow-Origin", "*")
			writer.Header().Set("Access-Control-Allow-Methods", http.MethodPut)
			writer.Header().Set("Access-Control-Allow-Headers", request.Header.Get("Access-Control-Request-Headers"))
		}
		writer.WriteHeader(http.StatusNoContent)
	case request.Method == http.MethodPut && strings.HasPrefix(request.URL.Path, "/cervi/organizations/"):
		if request.URL.Query().Get("X-Amz-Signature") == "" || request.Header.Get("Origin") != s3ProbeOrigin || request.Header.Get("Cache-Control") != serverfilecontent.ImmutableCacheControl {
			t.Errorf("invalid signed PUT request: %s %#v", request.URL.Path, request.Header)
		}
		payload, err := io.ReadAll(request.Body)
		if err != nil {
			t.Error(err)
		}
		s.mu.Lock()
		s.storageKey = strings.TrimPrefix(request.URL.Path, "/cervi/")
		s.payload = payload
		s.mu.Unlock()
		if s.putCORS {
			writer.Header().Set("Access-Control-Allow-Origin", "*")
		}
		writer.WriteHeader(s.putStatus)
	case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/public/organizations/"):
		if request.Header.Get("Authorization") != "" || request.URL.RawQuery != "" {
			t.Errorf("public GET contains authorization: %s %#v", request.URL.RawQuery, request.Header)
		}
		s.mu.Lock()
		payload := append([]byte(nil), s.payload...)
		storageKey := s.storageKey
		s.mu.Unlock()
		if request.URL.Path != "/public/"+storageKey {
			http.NotFound(writer, request)
			return
		}
		writer.WriteHeader(s.publicStatus)
		if s.publicStatus == http.StatusOK {
			_, _ = writer.Write(payload)
		}
	case request.Method == http.MethodDelete && strings.HasPrefix(request.URL.Path, "/cervi/organizations/"):
		if request.Header.Get("Authorization") == "" {
			t.Error("missing signed DeleteObject authorization")
		}
		writer.WriteHeader(s.deleteStatus)
		if s.deleteStatus >= http.StatusOK && s.deleteStatus < http.StatusMultipleChoices {
			s.mu.Lock()
			s.deleted = true
			s.mu.Unlock()
		}
	default:
		t.Errorf("unexpected request: %s %s", request.Method, request.URL.Path)
		http.NotFound(writer, request)
	}
}

// testS3Setting 返回测试服务使用的对象存储配置。
func testS3Setting(serverURL string) S3Setting {
	return S3Setting{
		Enabled: true, Provider: domain.StorageProviderGeneric,
		Endpoint: serverURL, PublicBaseURL: serverURL + "/public", Region: "us-east-1", Bucket: "cervi",
		AccessKeyID: "access-key", SecretAccessKey: "secret-key", ForcePathStyle: true,
	}
}
