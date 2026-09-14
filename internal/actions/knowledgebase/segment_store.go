//go:build server

package knowledgebase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/common/searchtext"
	"github.com/runforyou-ai/cervi/internal/common/textsplit"
	"github.com/uptrace/bun"
	"uuid"
)

const (
	// segmentInsertSize 是单条插入语句写入的分段数量，受 PostgreSQL 绑定参数上限约束。
	segmentInsertSize = 500
	// segmentCandidateLimit 是词法路和向量路各自返回的候选数量。
	segmentCandidateLimit = 50
	// lexicalMatchLimit 是词法路参与排名的最大命中数，超过时按近似排名处理。
	lexicalMatchLimit = 2000
)

// segmentHit 表示检索或阅读命中的一条已发布分段。
type segmentHit struct {
	ID             string `bun:"id"`
	DocumentID     string `bun:"document_id"`
	DocumentName   string `bun:"document_name"`
	SegmentBatchID string `bun:"segment_batch_id"`
	Position       int    `bun:"position"`
	Content        string `bun:"content"`
}

// segmentColumns 是检索和阅读结果读取的分段列。
const segmentColumns = "ks.id, ks.document_id, f.original_name AS document_name, ks.segment_batch_id, ks.position, ks.content"

// publishedSegments 只选取与文档当前已发布批次一致的分段，并联结文档名称。
func publishedSegments(db bun.IDB, knowledgeBaseID string) *bun.SelectQuery {
	return db.NewSelect().TableExpr("public.knowledge_segments AS ks").
		Join("JOIN knowledge_documents kd ON kd.id = ks.document_id AND kd.segment_batch_id = ks.segment_batch_id").
		Join("JOIN files f ON f.id = kd.file_id").
		Where("ks.knowledge_base_id = ?", knowledgeBaseID)
}

// insertSegments 按任务标识和文档内序号写入本批次分段、向量和词法词元。
func insertSegments(ctx context.Context, tx bun.IDB, input ProcessInput, segments []textsplit.Segment, vectors [][]float32) error {
	namespace, err := uuid.Parse(input.ProcessingID)
	if err != nil {
		return err
	}
	for start := 0; start < len(segments); start += segmentInsertSize {
		batch := segments[start:min(start+segmentInsertSize, len(segments))]
		placeholders := make([]string, 0, len(batch))
		arguments := make([]any, 0, len(batch)*10)
		for offset, segment := range batch {
			vector := make([]string, 0, input.EmbeddingDimension)
			for _, value := range vectors[start+offset] {
				vector = append(vector, strconv.FormatFloat(float64(value), 'f', -1, 32))
			}
			placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?::vector, ?, ?::tsvector)")
			arguments = append(arguments, common.NewUUIDv5(namespace, strconv.Itoa(segment.Position)).String(),
				input.OrganizationID, input.KnowledgeBaseID, input.DocumentID, input.ProcessingID,
				segment.Position, segment.CharacterCount, segment.Content,
				"["+strings.Join(vector, ",")+"]", input.EmbeddingDimension, searchtext.KnowledgeVector(segment.Content))
		}
		query := "INSERT INTO public.knowledge_segments (id, organization_id, knowledge_base_id, document_id, segment_batch_id, position, character_count, content, embedding, embedding_dimension, search_vector) VALUES " + strings.Join(placeholders, ", ")
		if _, err := tx.ExecContext(ctx, query, arguments...); err != nil {
			return err
		}
	}
	return nil
}

// deleteDocumentSegments 删除文档的全部分段。
func deleteDocumentSegments(ctx context.Context, tx bun.IDB, documentID string) error {
	_, err := tx.NewDelete().TableExpr("public.knowledge_segments").Where("document_id = ?", documentID).Exec(ctx)
	return err
}

// deleteKnowledgeBaseSegments 删除知识库的全部分段。
func deleteKnowledgeBaseSegments(ctx context.Context, tx bun.IDB, knowledgeBaseID string) error {
	_, err := tx.NewDelete().TableExpr("public.knowledge_segments").Where("knowledge_base_id = ?", knowledgeBaseID).Exec(ctx)
	return err
}

// searchSegmentsByVector 按余弦距离返回最近的已发布分段；维度以字面量写入以命中对应的部分索引。
func searchSegmentsByVector(ctx context.Context, db bun.IDB, knowledgeBaseID string, dimension int, vector []float32) ([]segmentHit, error) {
	values := make([]string, 0, len(vector))
	for _, value := range vector {
		values = append(values, strconv.FormatFloat(float64(value), 'f', -1, 32))
	}
	hits := make([]segmentHit, 0, segmentCandidateLimit)
	err := publishedSegments(db, knowledgeBaseID).ColumnExpr(segmentColumns).
		Where(fmt.Sprintf("ks.embedding_dimension = %d", dimension)).
		OrderExpr(fmt.Sprintf("ks.embedding::halfvec(%d) <=> ?::halfvec(%d)", dimension, dimension), "["+strings.Join(values, ",")+"]").
		Limit(segmentCandidateLimit).Scan(ctx, &hits)
	return hits, err
}

// searchSegmentsByText 在候选上限内按覆盖密度排名词法命中的已发布分段。
func searchSegmentsByText(ctx context.Context, db bun.IDB, knowledgeBaseID, tsquery string) ([]segmentHit, error) {
	candidates := publishedSegments(db, knowledgeBaseID).ColumnExpr(segmentColumns).ColumnExpr("ks.search_vector").
		Where("ks.search_vector @@ ?::tsquery", tsquery).Limit(lexicalMatchLimit)
	hits := make([]segmentHit, 0, segmentCandidateLimit)
	err := db.NewSelect().With("candidates", candidates).TableExpr("candidates").
		ColumnExpr("id, document_id, document_name, segment_batch_id, position, content").
		OrderExpr("ts_rank_cd(search_vector, ?::tsquery) DESC, id", tsquery).
		Limit(segmentCandidateLimit).Scan(ctx, &hits)
	return hits, err
}

// readSegmentWindow 读取指定分段及其前后相邻分段；分段不属于文档当前已发布批次时返回 ErrSegmentStale。
func readSegmentWindow(ctx context.Context, db bun.IDB, knowledgeBaseID, documentID, segmentID string, before, after int) ([]segmentHit, error) {
	if !common.ValidUUID(documentID) || !common.ValidUUID(segmentID) {
		return nil, ErrSegmentStale
	}
	var position int
	err := publishedSegments(db, knowledgeBaseID).ColumnExpr("ks.position").
		Where("ks.document_id = ? AND ks.id = ?", documentID, segmentID).Scan(ctx, &position)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSegmentStale
	}
	if err != nil {
		return nil, err
	}
	hits := make([]segmentHit, 0, before+after+1)
	err = publishedSegments(db, knowledgeBaseID).ColumnExpr(segmentColumns).
		Where("ks.document_id = ? AND ks.position BETWEEN ? AND ?", documentID, position-before, position+after).
		OrderExpr("ks.position").Scan(ctx, &hits)
	return hits, err
}
