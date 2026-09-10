package knowledgeprocessing

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestClientStreamsOriginalAndReadsAnchor 验证流式原件与锚点请求的内部契约。
func TestClientStreamsOriginalAndReadsAnchor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/knowledge/process":
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			defer file.Close()
			defer r.MultipartForm.RemoveAll()
			content, _ := io.ReadAll(file)
			var input struct {
				ProcessInput
				Embedding EmbeddingCredential `json:"embedding"`
			}
			if err := json.Unmarshal([]byte(r.FormValue("metadata")), &input); err != nil {
				t.Error(err)
			}
			if header.Filename != "资料.txt" || string(content) != "合同正文" || input.ChunkLength != 512 || input.Embedding.BaseURL != "https://models.test/v1" || input.Embedding.APIKey != "test-key" {
				t.Errorf("unexpected upload: %+v %q", input, content)
			}
			_, _ = w.Write([]byte(`{"segmentCount":2,"stale":false}`))
		case "/knowledge/segments":
			var input map[string]any
			_ = json.NewDecoder(r.Body).Decode(&input)
			if _, exists := input["anchorSegmentId"]; exists {
				t.Error("empty anchor must be omitted")
			}
			_, _ = w.Write([]byte(`{"segmentBatchId":"batch","page":1,"pageSize":20,"total":2,"segments":[]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.Process(context.Background(), ProcessInput{ChunkLength: 512}, EmbeddingCredential{BaseURL: "https://models.test/v1", APIKey: "test-key"}, "资料.txt", strings.NewReader("合同正文"))
	if err != nil || result.SegmentCount != 2 {
		t.Fatalf("process=%+v %v", result, err)
	}
	page, err := client.List(context.Background(), ListInput{Page: 1, PageSize: 20})
	if err != nil || page.Total != 2 {
		t.Fatalf("page=%+v %v", page, err)
	}
}

// TestClientFailureDoesNotExposeRemoteBody 验证未知错误响应映射为服务失败原因码。
func TestClientFailureDoesNotExposeRemoteBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("secret document body"))
	}))
	defer server.Close()
	_, err := NewClient(server.URL).List(context.Background(), ListInput{})
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != "service_failed" || strings.Contains(err.Error(), "secret") {
		t.Fatalf("error=%v", err)
	}
	_, err = NewClient("").Process(context.Background(), ProcessInput{}, EmbeddingCredential{}, "sample.txt", strings.NewReader("text"))
	if !errors.As(err, &failure) || failure.Code != "unavailable" {
		t.Fatalf("error=%v", err)
	}
}

// TestClientConnectionCheck 验证单次连接检查、不可用服务与超时分类。
func TestClientConnectionCheck(t *testing.T) {
	for _, test := range []struct {
		name    string
		status  int
		timeout bool
		code    string
	}{{"ready", 200, false, ""}, {"unavailable", 503, false, "unavailable"}, {"timeout", 200, true, "connection_timeout"}} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodGet || r.URL.Path != "/status" {
					t.Errorf("request=%s %s", r.Method, r.URL.Path)
				}
				if test.timeout {
					<-r.Context().Done()
					return
				}
				w.WriteHeader(test.status)
			}))
			defer server.Close()
			client := NewClient(server.URL)
			if test.timeout {
				client.http.Timeout = 50 * time.Millisecond
			}
			err := client.CheckConnection(context.Background())
			if test.code == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				var failure *Error
				if !errors.As(err, &failure) || failure.Code != test.code {
					t.Fatalf("failure=%v", err)
				}
			}
			if calls.Load() != 1 {
				t.Fatalf("calls=%d", calls.Load())
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()
	for _, url := range []string{"", server.URL} {
		var failure *Error
		if err := NewClient(url).CheckConnection(context.Background()); !errors.As(err, &failure) || failure.Code != "unavailable" {
			t.Fatalf("failure=%v", err)
		}
	}
}

// TestClientProcessingTimeout 验证处理超时保留独立错误原因。
func TestClientProcessingTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.http.Timeout = 50 * time.Millisecond
	_, err := client.Process(context.Background(), ProcessInput{}, EmbeddingCredential{}, "text.txt", strings.NewReader("正文"))
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != "request_timeout" {
		t.Fatalf("failure=%v", err)
	}
}
