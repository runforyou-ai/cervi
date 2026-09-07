//go:build server

package knowledgebase

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/runforyou-ai/cervi/internal/integration/connector"
)

type fileDownloadStub struct {
	calls   atomic.Int32
	release chan struct{}
	failure bool
}

// DownloadFile 记录缓存测试中的实际下载次数。
func (s *fileDownloadStub) DownloadFile(ctx context.Context, _ connector.DifyKnowledgeBaseConfig, _, _ string) (*connector.DifyKnowledgeFile, error) {
	s.calls.Add(1)
	if s.release != nil {
		select {
		case <-s.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if s.failure {
		return nil, errors.New("download failed")
	}
	return &connector.DifyKnowledgeFile{Name: "readme.md", Content: []byte("正文")}, nil
}

// TestDocumentFileCache 验证并发合并、自动到期删除和企业隔离。
func TestDocumentFileCache(t *testing.T) {
	downloader := &fileDownloadStub{release: make(chan struct{})}
	query := NewGetKnowledgeDocumentFileQuery(nil, downloader)
	query.ttl = 50 * time.Millisecond
	key := documentFileCacheKey{organizationID: "one", knowledgeBaseID: "base", documentID: "doc"}
	var requests sync.WaitGroup
	for range 8 {
		requests.Go(func() {
			file, err := query.read(context.Background(), key, connector.DifyKnowledgeBaseConfig{}, "dataset")
			if err != nil || string(file.Content) != "正文" {
				t.Errorf("file=%v error=%v", file, err)
			}
		})
	}
	close(downloader.release)
	requests.Wait()
	if downloader.calls.Load() != 1 {
		t.Fatalf("duplicate downloads: %d", downloader.calls.Load())
	}
	foreign := key
	foreign.organizationID = "two"
	if _, err := query.read(context.Background(), foreign, connector.DifyKnowledgeBaseConfig{}, "dataset"); err != nil {
		t.Fatal(err)
	}
	if downloader.calls.Load() != 2 {
		t.Fatal("file cache crossed organization boundary")
	}
	// 无后续读取时，定时器也必须清除到期文件。
	deadline := time.Now().Add(time.Second)
	for {
		query.mu.Lock()
		count := len(query.files)
		query.mu.Unlock()
		if count == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expired files were retained")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := query.read(context.Background(), key, connector.DifyKnowledgeBaseConfig{}, "dataset"); err != nil {
		t.Fatal(err)
	}
	if downloader.calls.Load() != 3 {
		t.Fatal("expired file was not downloaded again")
	}
}

// TestDocumentFileFailureNotCached 验证失败下载可以重新执行。
func TestDocumentFileFailureNotCached(t *testing.T) {
	downloader := &fileDownloadStub{failure: true}
	query := NewGetKnowledgeDocumentFileQuery(nil, downloader)
	key := documentFileCacheKey{documentID: "doc"}
	for range 2 {
		if _, err := query.read(context.Background(), key, connector.DifyKnowledgeBaseConfig{}, "dataset"); err == nil {
			t.Fatal("expected download error")
		}
	}
	if downloader.calls.Load() != 2 || len(query.files) != 0 {
		t.Fatal("failed download was cached")
	}
}

// TestDocumentFileReaderCancellation 验证首个读取者取消后，共享下载仍可供其他读取者使用。
func TestDocumentFileReaderCancellation(t *testing.T) {
	downloader := &fileDownloadStub{release: make(chan struct{})}
	query := NewGetKnowledgeDocumentFileQuery(nil, downloader)
	key := documentFileCacheKey{documentID: "doc"}
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { _, err := query.read(ctx, key, connector.DifyKnowledgeBaseConfig{}, "dataset"); first <- err }()
	deadline := time.Now().Add(time.Second)
	for downloader.calls.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("download did not start")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("first reader: %v", err)
	}
	close(downloader.release)
	file, err := query.read(context.Background(), key, connector.DifyKnowledgeBaseConfig{}, "dataset")
	if err != nil || file == nil || string(file.Content) != "正文" || downloader.calls.Load() != 1 {
		t.Fatalf("shared download file=%v err=%v calls=%d", file, err, downloader.calls.Load())
	}
}
