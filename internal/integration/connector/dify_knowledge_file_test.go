package connector

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDifyKnowledgeFile 验证原文件读取、无原文件状态及下载凭据隔离。
func TestDifyKnowledgeFile(t *testing.T) {
	source := "upload_file"
	status := http.StatusOK
	var remote *httptest.Server
	downloads := 0
	remote = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/original" {
			downloads++
			if r.Header.Get("Authorization") != "" {
				t.Error("API key leaked to file host")
			}
			_, _ = w.Write([]byte("完整原文件"))
			return
		}
		if r.Header.Get("Authorization") != "Bearer private-key" {
			t.Error("missing API authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/datasets/dataset/documents/doc/download" {
			w.WriteHeader(status)
			if status == http.StatusOK {
				_, _ = fmt.Fprintf(w, `{"url":%q}`, "/original")
			}
			return
		}
		_, _ = fmt.Fprintf(w, `{"id":"doc","name":"重命名后的标题","display_status":"available","data_source_type":%q,"data_source_info":{"upload_file":{"name":"original.md","extension":"md"}}}`, source)
	}))
	defer remote.Close()
	reader := NewDifyKnowledgeDocumentLister(remote.Client())
	config := DifyKnowledgeBaseConfig{APIURL: remote.URL + "/v1", APIKey: "private-key"}
	file, err := reader.DownloadFile(context.Background(), config, "dataset", "doc")
	if err != nil || file.Name != "original.md" || string(file.Content) != "完整原文件" {
		t.Fatalf("file=%v err=%v", file, err)
	}
	for _, mode := range []string{"website_crawl", "notion_import", "upload_file"} {
		source = mode
		status = http.StatusNotFound
		file, err := reader.DownloadFile(context.Background(), config, "dataset", "doc")
		if err != nil || file != nil {
			t.Fatalf("unsupported source=%s file=%v err=%v", mode, file, err)
		}
	}
	if downloads != 1 {
		t.Fatal("unsupported source attempted download")
	}
	status = http.StatusServiceUnavailable
	if _, err := reader.DownloadFile(context.Background(), config, "dataset", "doc"); err == nil {
		t.Fatal("remote error was treated as unavailable preview")
	}
}
