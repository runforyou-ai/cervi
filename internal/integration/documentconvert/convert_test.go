//go:build server

package documentconvert

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestConvertStreamsOriginal 验证流式提交原件并取回 Markdown 正文。
func TestConvertStreamsOriginal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/convert" || r.Method != http.MethodPost {
			t.Errorf("request=%s %s", r.Method, r.URL.Path)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		defer file.Close()
		defer r.MultipartForm.RemoveAll()
		content, _ := io.ReadAll(file)
		if header.Filename != "合同.docx" || string(content) != "原件字节" {
			t.Errorf("upload=%q %q", header.Filename, content)
		}
		_, _ = w.Write([]byte(`{"markdown":"# 合同\n金额 1234.50 元。"}`))
	}))
	defer server.Close()
	markdown, err := NewClient(server.URL).Convert(context.Background(), "合同.docx", strings.NewReader("原件字节"))
	if err != nil || markdown != "# 合同\n金额 1234.50 元。" {
		t.Fatalf("markdown=%q %v", markdown, err)
	}
}

// TestConvertFailureCodes 验证约定原因码与未知响应体的映射。
func TestConvertFailureCodes(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
		code   string
	}{
		{"解析失败", 422, `{"detail":{"code":"parse_failed"}}`, "parse_failed"},
		{"未知响应", 500, "secret document body", "service_failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			_, err := NewClient(server.URL).Convert(context.Background(), "合同.pdf", strings.NewReader("原件"))
			var failure *Error
			if !errors.As(err, &failure) || failure.Code != test.code || strings.Contains(err.Error(), "secret") {
				t.Fatalf("error=%v", err)
			}
		})
	}
	// 空地址和已关闭的服务都归为不可用。
	closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	closed.Close()
	for _, url := range []string{"", closed.URL} {
		var failure *Error
		if _, err := NewClient(url).Convert(context.Background(), "合同.pdf", strings.NewReader("原件")); !errors.As(err, &failure) || failure.Code != "unavailable" {
			t.Fatalf("url=%q error=%v", url, err)
		}
	}
}

// TestConvertTimeout 验证转换超时保留独立原因码。
func TestConvertTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.http.Timeout = 50 * time.Millisecond
	_, err := client.Convert(context.Background(), "合同.pdf", strings.NewReader("原件"))
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != "request_timeout" {
		t.Fatalf("failure=%v", err)
	}
}

// TestCheckConnection 验证单次连接检查、不可用服务、超时与连接拒绝。
func TestCheckConnection(t *testing.T) {
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
	// 空地址和已关闭的服务都归为不可用。
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()
	for _, url := range []string{"", server.URL} {
		var failure *Error
		if err := NewClient(url).CheckConnection(context.Background()); !errors.As(err, &failure) || failure.Code != "unavailable" {
			t.Fatalf("failure=%v", err)
		}
	}
}
