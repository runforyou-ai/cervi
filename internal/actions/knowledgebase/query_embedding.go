//go:build server

package knowledgebase

import (
	"context"
	"fmt"
	"sync"

	"github.com/runforyou-ai/cervi/pkg/embedding"
)

// queryEmbeddingKey 标识一组可以共用查询向量的向量模型配置。
type queryEmbeddingKey struct {
	providerID string
	model      string
	dimension  int
}

// queryEmbedding 保存一条查询的向量化结果，done 关闭后 vector 与 err 可读。
type queryEmbedding struct {
	done   chan struct{}
	vector []float32
	err    error
}

// queryEmbeddings 为同一向量模型配置下的全部知识库批量向量化查询，并按查询文本共享结果。
type queryEmbeddings struct {
	embedder   queryEmbedder
	credential embedding.Credential
	key        queryEmbeddingKey

	mu      sync.Mutex
	results map[string]*queryEmbedding
}

// newQueryEmbeddings 创建一组向量模型配置的查询向量缓存。
func newQueryEmbeddings(embedder queryEmbedder, credential embedding.Credential, key queryEmbeddingKey) *queryEmbeddings {
	return &queryEmbeddings{embedder: embedder, credential: credential, key: key, results: map[string]*queryEmbedding{}}
}

// start 以一次模型调用在后台向量化尚未提交的查询，已提交的查询复用原结果。
func (q *queryEmbeddings) start(ctx context.Context, queries []string) {
	q.mu.Lock()
	batch := make([]string, 0, len(queries))
	pending := make([]*queryEmbedding, 0, len(queries))
	for _, query := range queries {
		if _, exists := q.results[query]; exists {
			continue
		}
		result := &queryEmbedding{done: make(chan struct{})}
		q.results[query] = result
		batch, pending = append(batch, query), append(pending, result)
	}
	q.mu.Unlock()
	if len(batch) == 0 {
		return
	}
	go func() {
		vectors, err := q.embedder.Embed(ctx, q.credential, q.key.model, q.key.dimension, batch)
		if err == nil && len(vectors) != len(batch) {
			err = fmt.Errorf("embedding returned %d vectors for %d queries", len(vectors), len(batch))
		}
		// 失败的查询移出缓存，后续检索重新提交。
		if err != nil {
			q.mu.Lock()
			for index, query := range batch {
				if q.results[query] == pending[index] {
					delete(q.results, query)
				}
			}
			q.mu.Unlock()
		}
		for index, result := range pending {
			if err != nil {
				result.err = err
			} else {
				result.vector = vectors[index]
			}
			close(result.done)
		}
	}()
}

// vector 等待并返回查询向量，尚未提交的查询先单独提交。
func (q *queryEmbeddings) vector(ctx context.Context, query string) ([]float32, error) {
	q.start(ctx, []string{query})
	q.mu.Lock()
	result := q.results[query]
	q.mu.Unlock()
	select {
	case <-result.done:
		return result.vector, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
