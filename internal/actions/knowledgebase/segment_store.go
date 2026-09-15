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
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
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

// segmentBatch 固定一次分段发布的企业、知识库、来源、批次和向量维度。
type segmentBatch struct {
	OrganizationID     string
	KnowledgeBaseID    string
	SourceType         domain.KnowledgeSourceType
	SourceID           string
	BatchID            string
	EmbeddingDimension int
}

// segmentHit 表示检索或阅读命中的一条已发布分段，来源名称为文档文件名或问答主问题。
type segmentHit struct {
	ID             string `bun:"id"`
	SourceID       string `bun:"source_id"`
	SourceName     string `bun:"source_name"`
	SegmentBatchID string `bun:"segment_batch_id"`
	Position       int    `bun:"position"`
	Content        string `bun:"content"`
}

// segmentColumns 返回检索和阅读结果读取的分段列，来源名称按知识库类别取文档文件名或问答主问题。
func segmentColumns(base servermodels.KnowledgeBase) string {
	name := "f.original_name"
	if base.Category == string(domain.KnowledgeBaseCategoryQA) {
		name = "question.content"
	}
	return "ks.id, ks.source_id, " + name + " AS source_name, ks.segment_batch_id, ks.position, ks.content"
}

// publishedSegments 按知识库类别联结来源记录，只选取与来源当前已发布批次一致的分段。
func publishedSegments(db bun.IDB, base servermodels.KnowledgeBase) *bun.SelectQuery {
	query := db.NewSelect().TableExpr("public.knowledge_segments AS ks").Where("ks.knowledge_base_id = ?", base.ID)
	if base.Category == string(domain.KnowledgeBaseCategoryQA) {
		return query.Join("JOIN knowledge_qa_entries kqe ON kqe.id = ks.source_id AND kqe.segment_batch_id = ks.segment_batch_id").
			Join("JOIN knowledge_qa_contents question ON question.entry_id = kqe.id AND question.kind = ?", domain.KnowledgeQAContentPrimaryQuestion)
	}
	return query.Join("JOIN knowledge_documents kd ON kd.id = ks.source_id AND kd.segment_batch_id = ks.segment_batch_id").
		Join("JOIN files f ON f.id = kd.file_id")
}

// insertSegments 按批次标识和来源内序号写入本批次分段、向量和词法词元。
func insertSegments(ctx context.Context, tx bun.IDB, batch segmentBatch, segments []textsplit.Segment, vectors [][]float32) error {
	namespace, err := uuid.Parse(batch.BatchID)
	if err != nil {
		return err
	}
	for start := 0; start < len(segments); start += segmentInsertSize {
		chunk := segments[start:min(start+segmentInsertSize, len(segments))]
		placeholders := make([]string, 0, len(chunk))
		arguments := make([]any, 0, len(chunk)*12)
		for offset, segment := range chunk {
			vector := make([]string, 0, batch.EmbeddingDimension)
			for _, value := range vectors[start+offset] {
				vector = append(vector, strconv.FormatFloat(float64(value), 'f', -1, 32))
			}
			placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?::vector, ?, ?::tsvector)")
			arguments = append(arguments, common.NewUUIDv5(namespace, strconv.Itoa(segment.Position)).String(),
				batch.OrganizationID, batch.KnowledgeBaseID, batch.SourceType, batch.SourceID, batch.BatchID,
				segment.Position, segment.CharacterCount, segment.Content,
				"["+strings.Join(vector, ",")+"]", batch.EmbeddingDimension, searchtext.KnowledgeVector(segment.Content))
		}
		query := "INSERT INTO public.knowledge_segments (id, organization_id, knowledge_base_id, source_type, source_id, segment_batch_id, position, character_count, content, embedding, embedding_dimension, search_vector) VALUES " + strings.Join(placeholders, ", ")
		if _, err := tx.ExecContext(ctx, query, arguments...); err != nil {
			return err
		}
	}
	return nil
}

// deleteSourceSegments 删除一个文档或问答条目的全部分段。
func deleteSourceSegments(ctx context.Context, tx bun.IDB, sourceID string) error {
	_, err := tx.NewDelete().TableExpr("public.knowledge_segments").Where("source_id = ?", sourceID).Exec(ctx)
	return err
}

// deleteKnowledgeBaseSegments 删除知识库的全部分段。
func deleteKnowledgeBaseSegments(ctx context.Context, tx bun.IDB, knowledgeBaseID string) error {
	_, err := tx.NewDelete().TableExpr("public.knowledge_segments").Where("knowledge_base_id = ?", knowledgeBaseID).Exec(ctx)
	return err
}

// searchSegmentsByVector 按余弦距离返回最近的已发布分段；维度以字面量写入以命中对应的部分索引。
func searchSegmentsByVector(ctx context.Context, db bun.IDB, base servermodels.KnowledgeBase, vector []float32) ([]segmentHit, error) {
	values := make([]string, 0, len(vector))
	for _, value := range vector {
		values = append(values, strconv.FormatFloat(float64(value), 'f', -1, 32))
	}
	hits := make([]segmentHit, 0, segmentCandidateLimit)
	err := publishedSegments(db, base).ColumnExpr(segmentColumns(base)).
		Where(fmt.Sprintf("ks.embedding_dimension = %d", base.EmbeddingDimension)).
		OrderExpr(fmt.Sprintf("ks.embedding::halfvec(%d) <=> ?::halfvec(%d)", base.EmbeddingDimension, base.EmbeddingDimension), "["+strings.Join(values, ",")+"]").
		Limit(segmentCandidateLimit).Scan(ctx, &hits)
	return hits, err
}

// searchSegmentsByText 在候选上限内按覆盖密度排名词法命中的已发布分段。
func searchSegmentsByText(ctx context.Context, db bun.IDB, base servermodels.KnowledgeBase, tsquery string) ([]segmentHit, error) {
	candidates := publishedSegments(db, base).ColumnExpr(segmentColumns(base)).ColumnExpr("ks.search_vector").
		Where("ks.search_vector @@ ?::tsquery", tsquery).Limit(lexicalMatchLimit)
	hits := make([]segmentHit, 0, segmentCandidateLimit)
	err := db.NewSelect().With("candidates", candidates).TableExpr("candidates").
		ColumnExpr("id, source_id, source_name, segment_batch_id, position, content").
		OrderExpr("ts_rank_cd(search_vector, ?::tsquery) DESC, id", tsquery).
		Limit(segmentCandidateLimit).Scan(ctx, &hits)
	return hits, err
}

// readSegmentWindow 读取指定分段及其前后相邻分段；分段不属于来源当前已发布批次时返回 ErrSegmentStale。
func readSegmentWindow(ctx context.Context, db bun.IDB, base servermodels.KnowledgeBase, sourceID, segmentID, batchID string, before, after int) ([]segmentHit, error) {
	if !common.ValidUUID(sourceID) || !common.ValidUUID(segmentID) || !common.ValidUUID(batchID) {
		return nil, ErrSegmentStale
	}
	var position int
	err := publishedSegments(db, base).ColumnExpr("ks.position").
		Where("ks.source_id = ? AND ks.id = ? AND ks.segment_batch_id = ?", sourceID, segmentID, batchID).Scan(ctx, &position)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSegmentStale
	}
	if err != nil {
		return nil, err
	}
	hits := make([]segmentHit, 0, before+after+1)
	err = publishedSegments(db, base).ColumnExpr(segmentColumns(base)).
		Where("ks.source_id = ? AND ks.segment_batch_id = ? AND ks.position BETWEEN ? AND ?", sourceID, batchID, position-before, position+after).
		OrderExpr("ks.position").Scan(ctx, &hits)
	return hits, err
}
