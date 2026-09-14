//go:build server

package knowledgebase

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"
	"unicode/utf8"

	"github.com/runforyou-ai/cervi/internal/common/textsplit"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/embedding"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

const ProcessQAEntryActionName = "knowledge.qa_entry.process"

// ProcessQAInput 固定本次问答索引任务的来源和向量参数，问题与答案在执行时读取。
type ProcessQAInput struct {
	OrganizationID           string `json:"organizationId"`
	KnowledgeBaseID          string `json:"knowledgeBaseId"`
	EntryID                  string `json:"entryId"`
	ProcessingID             string `json:"processingId"`
	EmbeddingProviderID      string `json:"embeddingProviderId"`
	EmbeddingModelIdentifier string `json:"embeddingModelIdentifier"`
	EmbeddingDimension       int    `json:"embeddingDimension"`
}

// ProcessQAEntryAction 把问答条目的主问题、相似问题和答案分段、向量化并发布批次。
type ProcessQAEntryAction struct {
	db       *bun.DB
	embedder segmentEmbedder
}

// NewProcessQAEntryAction 创建问答索引任务。
func NewProcessQAEntryAction(db *bun.DB, embedder segmentEmbedder) *ProcessQAEntryAction {
	return &ProcessQAEntryAction{db: db, embedder: embedder}
}

// Execute 执行当前问答任务，并在同一事务中写入分段与发布批次。
func (a *ProcessQAEntryAction) Execute(ctx context.Context, input ProcessQAInput) error {
	started := time.Now()
	current, err := a.setStage(ctx, input, domain.KnowledgeIndexSplitting)
	if err != nil || !current {
		return err
	}
	slog.Info("知识问答索引开始", "entry_id", input.EntryID, "processing_id", input.ProcessingID)
	credential, err := resolveEmbeddingCredential(ctx, a.db, input.OrganizationID, input.EmbeddingProviderID)
	var unavailable *embedding.Error
	if errors.As(err, &unavailable) {
		return &ProcessError{Code: unavailable.Code, Stage: domain.KnowledgeIndexEmbedding}
	}
	if err != nil {
		return err
	}
	// 主问题和相似问题各成一段，答案按固定长度切段，位置在批次内连续。
	contents := make([]servermodels.KnowledgeQAContent, 0)
	err = a.db.NewSelect().Model(&contents).Where("kqc.entry_id = ?", input.EntryID).
		OrderExpr("CASE kqc.kind WHEN ? THEN 0 WHEN ? THEN 1 ELSE 2 END, kqc.sort_order, kqc.id", domain.KnowledgeQAContentPrimaryQuestion, domain.KnowledgeQAContentSimilarQuestion).Scan(ctx)
	if err != nil {
		return err
	}
	segments := make([]textsplit.Segment, 0, len(contents))
	for _, content := range contents {
		if content.Kind == domain.KnowledgeQAContentAnswer {
			for _, segment := range textsplit.Split(content.Content, domain.KnowledgeQAChunkLength, domain.KnowledgeQAChunkOverlap) {
				segments = append(segments, textsplit.Segment{Position: len(segments) + 1, Content: segment.Content, CharacterCount: segment.CharacterCount})
			}
			continue
		}
		segments = append(segments, textsplit.Segment{Position: len(segments) + 1, Content: content.Content, CharacterCount: utf8.RuneCountInString(content.Content)})
	}
	if len(segments) == 0 {
		return &ProcessError{Code: "empty_content", Stage: domain.KnowledgeIndexSplitting}
	}

	if current, err := a.setStage(ctx, input, domain.KnowledgeIndexEmbedding); err != nil || !current {
		return err
	}
	texts := make([]string, 0, len(segments))
	for _, segment := range segments {
		texts = append(texts, segment.Content)
	}
	vectors, err := a.embedder.Embed(ctx, credential, input.EmbeddingModelIdentifier, input.EmbeddingDimension, texts)
	if err != nil {
		var failure *embedding.Error
		if errors.As(err, &failure) {
			return &ProcessError{Code: failure.Code, Stage: domain.KnowledgeIndexEmbedding}
		}
		return err
	}
	if len(vectors) != len(segments) {
		return &ProcessError{Code: "embedding_failed", Stage: domain.KnowledgeIndexEmbedding}
	}

	if current, err := a.setStage(ctx, input, domain.KnowledgeIndexPublishing); err != nil || !current {
		return err
	}
	published := false
	// 持有条目行锁校验当前任务，并在同一事务中替换分段与发布批次。
	err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		entry := &servermodels.KnowledgeQAEntry{}
		if err := tx.NewSelect().Model(entry).Where("kqe.id = ?", input.EntryID).For("UPDATE").Scan(ctx); errors.Is(err, sql.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}
		if entry.ProcessingID != input.ProcessingID || !entry.Status.IsProcessing() {
			return nil
		}
		if err := deleteSourceSegments(ctx, tx, input.EntryID); err != nil {
			return err
		}
		batch := segmentBatch{OrganizationID: input.OrganizationID, KnowledgeBaseID: input.KnowledgeBaseID, SourceType: domain.KnowledgeSourceQAEntry, SourceID: input.EntryID, BatchID: input.ProcessingID, EmbeddingDimension: input.EmbeddingDimension}
		if err := insertSegments(ctx, tx, batch, segments, vectors); err != nil {
			return err
		}
		_, err := tx.NewUpdate().Model(entry).Set("status = ?", domain.KnowledgeIndexSucceeded).Set("segment_batch_id = ?", input.ProcessingID).Set("segment_count = ?", len(segments)).Set("failure_code = ''").Set("updated_at = now()").WherePK().Exec(ctx)
		if err != nil {
			return err
		}
		if err := servertask.LockExecution(ctx, tx); err != nil {
			return err
		}
		published = true
		return nil
	})
	if err == nil && published {
		slog.Info("知识问答分段与向量完成", "entry_id", input.EntryID, "processing_id", input.ProcessingID, "segment_count", len(segments), "embedding_dimension", input.EmbeddingDimension, "duration_ms", time.Since(started).Milliseconds())
	}
	return err
}

// setStage 更新当前问答任务的执行阶段，任务已被替代或已进入终态时返回 false。
func (a *ProcessQAEntryAction) setStage(ctx context.Context, input ProcessQAInput, stage domain.KnowledgeIndexStatus) (bool, error) {
	return updateIndexStage(ctx, a.db, (*servermodels.KnowledgeQAEntry)(nil), input.EntryID, input.ProcessingID, stage)
}

// FinalizeFailure 保存当前问答任务的失败状态和原因码。
func (a *ProcessQAEntryAction) FinalizeFailure(ctx context.Context, input ProcessQAInput, runErr error) error {
	code, stage := indexFailureCode(runErr)
	changed, err := finalizeIndexFailure(ctx, a.db, (*servermodels.KnowledgeQAEntry)(nil), input.EntryID, input.ProcessingID, code)
	if err == nil && changed {
		slog.Warn("知识问答索引失败", "entry_id", input.EntryID, "processing_id", input.ProcessingID, "failure_code", code, "stage", stage)
	}
	return err
}
