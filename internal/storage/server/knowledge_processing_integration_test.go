//go:build server

package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"uuid"

	knowledgeaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeprocessing"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

type processingProbe struct {
	db            *bun.DB
	fail          bool
	connectionErr error
}

// CheckConnection 返回预设的连接检查结果。
func (p *processingProbe) CheckConnection(context.Context) error {
	return p.connectionErr
}

// Open 为执行任务提供固定原件。
func (p *processingProbe) Open(context.Context, *servermodels.File) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("原件")), nil
}

// Process 模拟远端持久化，同时验证发布所需的实际分段数量。
func (p *processingProbe) Process(ctx context.Context, input knowledgeprocessing.ProcessInput, _ string, _ io.Reader) (knowledgeprocessing.ProcessResult, error) {
	if p.fail {
		return knowledgeprocessing.ProcessResult{}, &knowledgeprocessing.Error{Code: "parse_failed", Stage: domain.KnowledgeDocumentExtracting}
	}
	meta, _ := json.Marshal(map[string]any{"document_id": input.DocumentID, "batch_id": input.ProcessingID, "position": 1})
	_, err := p.db.ExecContext(ctx, "INSERT INTO public.knowledge_segments(id,content,meta) VALUES (?, ?, ?::jsonb) ON CONFLICT(id) DO NOTHING", input.ProcessingID, "正文", string(meta))
	return knowledgeprocessing.ProcessResult{SegmentCount: 1}, err
}

// TestKnowledgeProcessingRetryAndPublication 验证上传投递、失败重试幂等、参数快照与完整发布。
func TestKnowledgeProcessingRetryAndPublication(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, testDatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	installed, base := newDocumentFixture(t, db)
	tasks := newDocumentTasks(t, db)
	file := uploadedDocumentFile(t, db, installed.Identity, "资料.txt")
	documents, err := knowledgeaction.NewCreateDocumentsAction(db, tasks).Execute(ctx, installed.Identity, base.ID, base.Groups[0].ID, []string{file.ID})
	if err != nil {
		t.Fatal(err)
	}
	documentID := documents[0].ID
	var document servermodels.KnowledgeDocument
	if err := db.NewSelect().Model(&document).Where("kd.id = ?", documentID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if document.Status != domain.KnowledgeDocumentQueued || document.ChunkLength != 512 || document.ProcessingID == "" {
		t.Fatalf("document=%+v", document)
	}
	input := knowledgeprocessing.ProcessInput{OrganizationID: installed.Identity.Organization.ID, KnowledgeBaseID: base.ID, DocumentID: documentID, ProcessingID: document.ProcessingID, ChunkLength: document.ChunkLength, ChunkOverlap: document.ChunkOverlap}
	probe := &processingProbe{db: db, fail: true}
	worker := knowledgeaction.NewProcessDocumentAction(db, probe, probe)
	err = worker.Execute(ctx, input)
	if err == nil {
		t.Fatalf("failure=%v", err)
	}
	if err := worker.FinalizeFailure(ctx, input, err); err != nil {
		t.Fatal(err)
	}
	query := knowledgeaction.NewDocumentQuery(db)
	failed, err := query.Get(ctx, installed.Identity, base.ID, documentID)
	if err != nil || failed.Status != domain.KnowledgeDocumentFailed || failed.FailureCode != "parse_failed" {
		t.Fatalf("failure=%+v %v", failed, err)
	}
	// 核验连续重试后生效的任务标识。
	retry := knowledgeaction.NewDocumentProcessing(db, tasks)
	var group sync.WaitGroup
	for range 2 {
		group.Go(func() {
			if err := retry.Retry(ctx, installed.Identity, base.ID, documentID, probe.CheckConnection); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	if err := db.NewSelect().Model(&document).Where("kd.id = ?", documentID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if document.ProcessingID == input.ProcessingID {
		t.Fatal("retry reused failed execution")
	}
	count, err := db.NewSelect().Model((*servermodels.TaskRun)(nil)).Where("action_name = ? AND payload->>'documentId' = ?", knowledgeaction.ProcessDocumentActionName, documentID).Count(ctx)
	if err != nil || count != 3 {
		t.Fatalf("tasks=%d %v", count, err)
	}
	// 核验旧任务成功和失败后的当前任务状态。
	if err := worker.Execute(ctx, input); err != nil {
		t.Fatal(err)
	}
	if err := worker.FinalizeFailure(ctx, input, errors.New("old failure")); err != nil {
		t.Fatal(err)
	}
	input.ProcessingID = document.ProcessingID
	probe.fail = false
	if err := worker.Execute(ctx, input); err != nil {
		t.Fatal(err)
	}
	if err := worker.Execute(ctx, input); err != nil {
		t.Fatal(err)
	}
	completed, err := query.Get(ctx, installed.Identity, base.ID, documentID)
	if err != nil || completed.Status != domain.KnowledgeDocumentSucceeded || completed.SegmentCount != 1 || completed.SegmentBatchID != input.ProcessingID {
		t.Fatalf("completed=%+v %v", completed, err)
	}
	if err := retry.Retry(ctx, installed.Identity, base.ID, documentID, probe.CheckConnection); err != nil {
		t.Fatal(err)
	}
	if err := knowledgeaction.NewDeleteDocumentAction(db).Execute(ctx, installed.Identity, base.ID, documentID); err != nil {
		t.Fatal(err)
	}
	count, err = db.NewSelect().TableExpr("public.knowledge_segments").Where("meta->>'document_id' = ?", documentID).Count(ctx)
	if err != nil || count != 0 {
		t.Fatalf("remaining=%d %v", count, err)
	}
	if err := worker.Execute(ctx, input); err != nil {
		t.Fatal(err)
	}
}

type segmentReaderProbe struct {
	page  knowledgeprocessing.SegmentPage
	calls int
}

// TestKnowledgeRetryAllStates 验证所有状态可按新配置重试，旧任务失效且已发布内容持续可读。
func TestKnowledgeRetryAllStates(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, testDatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	owner, base := newDocumentFixture(t, db)
	tasks := newDocumentTasks(t, db)
	docs, err := knowledgeaction.NewCreateDocumentsAction(db, tasks).Execute(ctx, owner.Identity, base.ID, base.Groups[0].ID, []string{uploadedDocumentFile(t, db, owner.Identity, "重新分段.txt").ID})
	if err != nil {
		t.Fatal(err)
	}
	var document servermodels.KnowledgeDocument
	if err := db.NewSelect().Model(&document).Where("kd.id = ?", docs[0].ID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	input := knowledgeprocessing.ProcessInput{OrganizationID: owner.Identity.Organization.ID, KnowledgeBaseID: base.ID, DocumentID: document.ID, ProcessingID: document.ProcessingID, ChunkLength: 512, ChunkOverlap: 50}
	probe := &processingProbe{db: db}
	worker := knowledgeaction.NewProcessDocumentAction(db, probe, probe)
	if err := worker.Execute(ctx, input); err != nil {
		t.Fatal(err)
	}
	published := input.ProcessingID
	if _, err := db.NewUpdate().Model((*servermodels.KnowledgeBase)(nil)).Set("chunk_length = 768").Set("chunk_overlap = 80").Where("id = ?", base.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	retry := knowledgeaction.NewDocumentProcessing(db, tasks)
	states := []domain.KnowledgeDocumentStatus{domain.KnowledgeDocumentInitial, domain.KnowledgeDocumentQueued, domain.KnowledgeDocumentFetching, domain.KnowledgeDocumentConverting, domain.KnowledgeDocumentExtracting, domain.KnowledgeDocumentRecognizing, domain.KnowledgeDocumentSplitting, domain.KnowledgeDocumentEmbedding, domain.KnowledgeDocumentIndexing, domain.KnowledgeDocumentPublishing, domain.KnowledgeDocumentSucceeded, domain.KnowledgeDocumentFailed, domain.KnowledgeDocumentCancelled}
	for _, state := range states {
		if _, err := db.NewUpdate().Model(&document).Set("status = ?", state).WherePK().Exec(ctx); err != nil {
			t.Fatal(err)
		}
		if err := retry.Retry(ctx, owner.Identity, base.ID, document.ID, probe.CheckConnection); err != nil {
			t.Fatalf("%s: %v", state, err)
		}
		if err := db.NewSelect().Model(&document).Where("kd.id = ?", document.ID).Scan(ctx); err != nil {
			t.Fatal(err)
		}
		if document.ProcessingID == input.ProcessingID || document.Status != domain.KnowledgeDocumentQueued || document.ChunkLength != 768 || document.ChunkOverlap != 80 || document.SegmentBatchID != published {
			t.Fatalf("%s: %+v", state, document)
		}
		if err := worker.Execute(ctx, input); err != nil {
			t.Fatal(err)
		}
		if err := worker.FinalizeFailure(ctx, input, errors.New("old failure")); err != nil {
			t.Fatal(err)
		}
		input.ProcessingID, input.ChunkLength, input.ChunkOverlap = document.ProcessingID, document.ChunkLength, document.ChunkOverlap
	}
	// 当前重试失败仍保留已完成分段，再次成功才替换正文。
	probe.fail = true
	if err := worker.FinalizeFailure(ctx, input, worker.Execute(ctx, input)); err != nil {
		t.Fatal(err)
	}
	count, err := db.NewSelect().TableExpr("public.knowledge_segments").Where("meta->>'document_id' = ? AND meta->>'batch_id' = ?", document.ID, published).Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("published=%d %v", count, err)
	}
	if err := retry.Retry(ctx, owner.Identity, base.ID, document.ID, probe.CheckConnection); err != nil {
		t.Fatal(err)
	}
	if err := db.NewSelect().Model(&document).Where("kd.id = ?", document.ID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	input.ProcessingID = document.ProcessingID
	probe.fail = false
	if err := worker.Execute(ctx, input); err != nil {
		t.Fatal(err)
	}
	count, err = db.NewSelect().TableExpr("public.knowledge_segments").Where("meta->>'document_id' = ? AND meta->>'batch_id' = ?", document.ID, published).Count(ctx)
	if err != nil || count != 0 {
		t.Fatalf("old batch=%d %v", count, err)
	}
	if err := db.NewSelect().Model(&document).Where("kd.id = ?", document.ID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if document.Status != domain.KnowledgeDocumentSucceeded || document.SegmentBatchID != input.ProcessingID {
		t.Fatalf("completed=%+v", document)
	}
}

// TestKnowledgeProcessingMissingFile 验证原件记录缺失会结束为可重试的失败状态。
func TestKnowledgeProcessingMissingFile(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, testDatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	owner, base := newDocumentFixture(t, db)
	file := uploadedDocumentFile(t, db, owner.Identity, "缺失原件.txt")
	docs, err := knowledgeaction.NewCreateDocumentsAction(db, newDocumentTasks(t, db)).Execute(ctx, owner.Identity, base.ID, base.Groups[0].ID, []string{file.ID})
	if err != nil {
		t.Fatal(err)
	}
	var document servermodels.KnowledgeDocument
	if err := db.NewSelect().Model(&document).Where("kd.id = ?", docs[0].ID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.NewDelete().Model(file).WherePK().Exec(ctx); err != nil {
		t.Fatal(err)
	}
	probe := &processingProbe{db: db}
	worker := knowledgeaction.NewProcessDocumentAction(db, probe, probe)
	input := knowledgeprocessing.ProcessInput{OrganizationID: owner.Identity.Organization.ID, KnowledgeBaseID: base.ID, DocumentID: document.ID, ProcessingID: document.ProcessingID}
	err = worker.Execute(ctx, input)
	if err == nil {
		t.Fatalf("failure=%v", err)
	}
	if err := worker.FinalizeFailure(ctx, input, err); err != nil {
		t.Fatal(err)
	}
	if err := db.NewSelect().Model(&document).Where("kd.id = ?", document.ID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if document.Status != domain.KnowledgeDocumentFailed || document.FailureCode != "file_read_failed" {
		t.Fatalf("document=%+v", document)
	}
}

// List 返回预设的远端分页响应。
func (p *segmentReaderProbe) List(context.Context, knowledgeprocessing.ListInput) (knowledgeprocessing.SegmentPage, error) {
	p.calls++
	return p.page, nil
}

// TestKnowledgeSegmentsScopeAndBatch 验证查询前后企业与批次校验。
func TestKnowledgeSegmentsScopeAndBatch(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, testDatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	owner, base := newDocumentFixture(t, db)
	docs, err := knowledgeaction.NewCreateDocumentsAction(db, newDocumentTasks(t, db)).Execute(ctx, owner.Identity, base.ID, base.Groups[0].ID, []string{uploadedDocumentFile(t, db, owner.Identity, "分段.txt").ID})
	if err != nil {
		t.Fatal(err)
	}
	batch := uuid.NewV7().String()
	_, err = db.NewUpdate().Model((*servermodels.KnowledgeDocument)(nil)).Set("segment_batch_id = ?", batch).Set("segment_count = 1").Where("id = ?", docs[0].ID).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}
	reader := &segmentReaderProbe{page: knowledgeprocessing.SegmentPage{SegmentBatchID: batch, Page: 1, PageSize: 20, Total: 1}}
	query := knowledgeaction.NewDocumentQuery(db)
	query.SetSegmentReader(reader)
	if _, err := query.Segments(ctx, owner.Identity, base.ID, docs[0].ID, knowledgeprocessing.ListInput{SegmentBatchID: uuid.NewV7().String()}); !errors.Is(err, knowledgeaction.ErrSegmentStale) {
		t.Fatalf("stale=%v", err)
	}
	if reader.calls != 0 {
		t.Fatal("stale query reached remote")
	}
	other, _ := newDocumentFixture(t, db)
	if _, err := query.Segments(ctx, other.Identity, base.ID, docs[0].ID, knowledgeprocessing.ListInput{}); err == nil {
		t.Fatal("foreign query allowed")
	}
	if reader.calls != 0 {
		t.Fatal("foreign query reached remote")
	}
	if _, err := query.Segments(ctx, owner.Identity, base.ID, docs[0].ID, knowledgeprocessing.ListInput{}); err != nil {
		t.Fatal(err)
	}
	reader.page.SegmentBatchID = uuid.NewV7().String()
	if _, err := query.Segments(ctx, owner.Identity, base.ID, docs[0].ID, knowledgeprocessing.ListInput{}); !errors.Is(err, knowledgeaction.ErrSegmentStale) {
		t.Fatalf("remote stale=%v", err)
	}
}

// TestKnowledgeConnectionFailureSkipsTask 验证连接失败状态、任务数量及过期任务校验。
func TestKnowledgeConnectionFailureSkipsTask(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, testDatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	owner, base := newDocumentFixture(t, db)
	tasks := newDocumentTasks(t, db)
	docs, err := knowledgeaction.NewCreateDocumentsAction(db, tasks).Execute(ctx, owner.Identity, base.ID, base.Groups[0].ID, []string{uploadedDocumentFile(t, db, owner.Identity, "服务离线.txt").ID})
	if err != nil {
		t.Fatal(err)
	}
	var document servermodels.KnowledgeDocument
	if err := db.NewSelect().Model(&document).Where("kd.id = ?", docs[0].ID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	input := knowledgeprocessing.ProcessInput{OrganizationID: owner.Identity.Organization.ID, KnowledgeBaseID: base.ID, DocumentID: document.ID, ProcessingID: document.ProcessingID}
	probe := &processingProbe{db: db}
	worker := knowledgeaction.NewProcessDocumentAction(db, probe, probe)
	if err := worker.Execute(ctx, input); err != nil {
		t.Fatal(err)
	}
	published := input.ProcessingID
	retry := knowledgeaction.NewDocumentProcessing(db, tasks)
	for _, code := range []string{"unavailable", "connection_timeout"} {
		probe.connectionErr = &knowledgeprocessing.Error{Code: code}
		if err := retry.Retry(ctx, owner.Identity, base.ID, document.ID, probe.CheckConnection); !errors.Is(err, probe.connectionErr) {
			t.Fatalf("failure=%v", err)
		}
		if err := db.NewSelect().Model(&document).Where("kd.id = ?", document.ID).Scan(ctx); err != nil {
			t.Fatal(err)
		}
		if document.Status != domain.KnowledgeDocumentFailed || document.FailureCode != code || document.ProcessingID == input.ProcessingID || document.SegmentBatchID != published || document.SegmentCount != 1 {
			t.Fatalf("document=%+v", document)
		}
		if err := worker.Execute(ctx, input); err != nil {
			t.Fatal(err)
		}
		if err := worker.FinalizeFailure(ctx, input, errors.New("late failure")); err != nil {
			t.Fatal(err)
		}
		input.ProcessingID = document.ProcessingID
	}
	var runs []servermodels.TaskRun
	if err := db.NewSelect().Model(&runs).Where("payload->>'documentId' = ?", document.ID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].MaxAttempts != 1 {
		t.Fatalf("runs=%+v", runs)
	}
	probe.connectionErr = nil
	if err := retry.Retry(ctx, owner.Identity, base.ID, document.ID, probe.CheckConnection); err != nil {
		t.Fatal(err)
	}
	if err := db.NewSelect().Model(&runs).Where("payload->>'documentId' = ?", document.ID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 || runs[1].MaxAttempts != 1 {
		t.Fatalf("runs=%+v", runs)
	}
	if err := db.NewSelect().Model(&document).Where("kd.id = ?", document.ID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	input.ProcessingID = document.ProcessingID
	probe.fail = true
	if err := worker.FinalizeFailure(ctx, input, worker.Execute(ctx, input)); err != nil {
		t.Fatal(err)
	}
	if err := db.NewSelect().Model(&document).Where("kd.id = ?", document.ID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if document.Status != domain.KnowledgeDocumentFailed || document.FailureCode != "parse_failed" || document.SegmentBatchID != published {
		t.Fatalf("document=%+v", document)
	}
}
