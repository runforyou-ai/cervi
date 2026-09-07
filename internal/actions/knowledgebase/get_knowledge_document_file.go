//go:build server

package knowledgebase

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/runforyou-ai/cervi/internal/integration/connector"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

type difyKnowledgeFileDownloader interface {
	DownloadFile(context.Context, connector.DifyKnowledgeBaseConfig, string, string) (*connector.DifyKnowledgeFile, error)
}

type documentFileCacheKey struct {
	organizationID, knowledgeBaseID, documentID string
	connection                                  [32]byte
}

type documentFileCacheEntry struct {
	ready     chan struct{}
	file      *DocumentFileRecord
	err       error
	expiresAt time.Time
}

// DocumentFileRecord 保存用于预览的原始文件。
type DocumentFileRecord struct {
	Name    string
	Content []byte
}

// GetKnowledgeDocumentFileQuery 按企业隔离缓存原文件，下载完成一小时后清除。
type GetKnowledgeDocumentFileQuery struct {
	db         *bun.DB
	downloader difyKnowledgeFileDownloader
	mu         sync.Mutex
	files      map[documentFileCacheKey]*documentFileCacheEntry
	ttl        time.Duration
}

// NewGetKnowledgeDocumentFileQuery 创建原文件读取查询。
func NewGetKnowledgeDocumentFileQuery(db *bun.DB, downloader difyKnowledgeFileDownloader) *GetKnowledgeDocumentFileQuery {
	return &GetKnowledgeDocumentFileQuery{db: db, downloader: downloader, files: make(map[documentFileCacheKey]*documentFileCacheEntry), ttl: time.Hour}
}

// Execute 校验当前企业的知识库归属，再读取缓存或下载原文件。
func (q *GetKnowledgeDocumentFileQuery) Execute(ctx context.Context, identity *servermodels.Identity, knowledgeBaseID, documentID string) (*DocumentFileRecord, error) {
	documentID = strings.TrimSpace(documentID)
	access, err := loadDifyKnowledgeAccess(ctx, q.db, identity.Organization.ID, knowledgeBaseID)
	if err != nil {
		return nil, err
	}
	if documentID == "" {
		return nil, ErrDocumentNotFound
	}
	key := documentFileCacheKey{organizationID: identity.Organization.ID, knowledgeBaseID: knowledgeBaseID, documentID: documentID,
		connection: sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s", access.Config.APIURL, access.Config.APIKey, access.DatasetID)))}
	return q.read(ctx, key, access.Config, access.DatasetID)
}

// read 合并同一原文件的并发下载，并按首次下载完成时间清除缓存。
func (q *GetKnowledgeDocumentFileQuery) read(ctx context.Context, key documentFileCacheKey, config connector.DifyKnowledgeBaseConfig, datasetID string) (*DocumentFileRecord, error) {
	q.mu.Lock()
	entry := q.files[key]
	if entry != nil && !entry.expiresAt.IsZero() && !time.Now().Before(entry.expiresAt) {
		delete(q.files, key)
		entry = nil
	}
	if entry == nil {
		entry = &documentFileCacheEntry{ready: make(chan struct{})}
		q.files[key] = entry
		go q.download(context.WithoutCancel(ctx), key, config, datasetID, entry)
	}
	q.mu.Unlock()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-entry.ready:
		return entry.file, entry.err
	}
}

// download 独立完成共享下载，单个读取者离开不会中断其他读取者。
func (q *GetKnowledgeDocumentFileQuery) download(ctx context.Context, key documentFileCacheKey, config connector.DifyKnowledgeBaseConfig, datasetID string, entry *documentFileCacheEntry) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	downloaded, err := q.downloader.DownloadFile(ctx, config, datasetID, key.documentID)
	var file *DocumentFileRecord
	if downloaded != nil {
		file = &DocumentFileRecord{Name: downloaded.Name, Content: downloaded.Content}
	}
	q.mu.Lock()
	entry.file, entry.err = file, err
	if err != nil || file == nil {
		delete(q.files, key)
	} else {
		entry.expiresAt = time.Now().Add(q.ttl)
		time.AfterFunc(q.ttl, func() {
			q.mu.Lock()
			defer q.mu.Unlock()
			if q.files[key] == entry {
				delete(q.files, key)
			}
		})
		slog.Info("知识文档原文件已缓存", "organization_id", key.organizationID, "knowledge_base_id", key.knowledgeBaseID, "document_id", key.documentID, "size", len(file.Content), "expires_at", entry.expiresAt)
	}
	close(entry.ready)
	q.mu.Unlock()
}
