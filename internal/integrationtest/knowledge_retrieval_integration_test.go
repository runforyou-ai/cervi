//go:build server

package integrationtest

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	knowledgeaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/integration/embedding"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
	"github.com/runforyou-ai/cervi/internal/integration/rerank"
	servertest "github.com/runforyou-ai/cervi/internal/servertest"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// retrievalTopics 按关键词把文本映射到向量分量，让向量路只在同主题文本之间距离为零。
var retrievalTopics = []string{"退款", "发票", "配送"}

type retrievalProbe struct {
	markdown  string
	embedFail bool
	reranked  int
}

// Open 提供固定原件。
func (p *retrievalProbe) Open(context.Context, *servermodels.File) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("原件")), nil
}

// Convert 返回当前预设正文。
func (p *retrievalProbe) Convert(context.Context, string, io.Reader) (string, error) {
	return p.markdown, nil
}

// Embed 按文本包含的主题关键词生成单位向量，未命中主题的文本落在独立分量。
func (p *retrievalProbe) Embed(_ context.Context, _ embedding.Credential, _ string, dimension int, inputs []string) ([][]float32, error) {
	if p.embedFail {
		return nil, &embedding.Error{Code: "embedding_failed"}
	}
	vectors := make([][]float32, len(inputs))
	for index, input := range inputs {
		vectors[index] = make([]float32, dimension)
		component := len(retrievalTopics)
		for topic, keyword := range retrievalTopics {
			if strings.Contains(input, keyword) {
				component = topic
			}
		}
		vectors[index][component] = 1
	}
	return vectors, nil
}

// Rerank 记录调用次数，并给靠后的候选更高的相关性以验证重排结果生效。
func (p *retrievalProbe) Rerank(_ context.Context, _ rerank.Credential, _, _ string, documents []string, _ int) ([]rerank.Score, error) {
	p.reranked++
	scores := make([]rerank.Score, 0, len(documents))
	for index := range documents {
		scores = append(scores, rerank.Score{Index: index, Relevance: float64(index+1) / float64(len(documents))})
	}
	return scores, nil
}

// publishRetrievalDocument 上传并处理一篇文档，返回文档编号。
func publishRetrievalDocument(t *testing.T, db *bun.DB, probe *retrievalProbe, identity *servermodels.Identity, base *knowledgeaction.Record, name, markdown string) string {
	t.Helper()
	ctx := context.Background()
	file := uploadedDocumentFile(t, db, identity, name)
	documents, err := knowledgeaction.NewCreateDocumentsAction(db, newDocumentTasks(t, db)).Execute(ctx, identity, base.ID, base.Groups[0].ID, []string{file.ID})
	if err != nil {
		t.Fatal(err)
	}
	var document servermodels.KnowledgeDocument
	if err := db.NewSelect().Model(&document).Where("kd.id = ?", documents[0].ID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	probe.markdown = markdown
	worker := knowledgeaction.NewProcessDocumentAction(db, probe, probe, probe)
	err = worker.Execute(ctx, knowledgeaction.ProcessInput{
		OrganizationID: identity.Organization.ID, KnowledgeBaseID: base.ID, DocumentID: document.ID, ProcessingID: document.ProcessingID,
		ChunkLength: document.ChunkLength, ChunkOverlap: document.ChunkOverlap,
		EmbeddingProviderID: document.EmbeddingProviderID, EmbeddingModelIdentifier: document.EmbeddingModelIdentifier, EmbeddingDimension: document.EmbeddingDimension,
	})
	if err != nil {
		t.Fatal(err)
	}
	return document.ID
}

// TestKnowledgeHybridRetrieval 验证词法与向量两路召回、名次融合、重排、单路失败保留、企业隔离与游标阅读。
func TestKnowledgeHybridRetrieval(t *testing.T) {
	ctx := context.Background()
	store, err := serverstorage.Open(ctx, servertest.DatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	installed, base := newDocumentFixture(t, db)
	identity := installed.Identity
	probe := &retrievalProbe{}
	service := knowledgeaction.NewRetrievalService(db, probe, probe)

	if _, err := service.Retrieve(ctx, identity, base.ID, "退款"); !errors.Is(err, knowledgeaction.ErrRetrievalNotReady) {
		t.Fatalf("err=%v", err)
	}
	refundID := publishRetrievalDocument(t, db, probe, identity, base, "退款政策.txt", strings.Repeat("签收后七天内可以申请退款，退款金额原路返回。", 30))
	publishRetrievalDocument(t, db, probe, identity, base, "发票说明.txt", "下单时可以选择开具电子发票。")
	publishRetrievalDocument(t, db, probe, identity, base, "配送说明.txt", "配送时效按收货地址计算。")

	// 词法路与向量路都命中退款文档，融合后排在首位。
	records, err := service.Retrieve(ctx, identity, base.ID, "如何申请退款")
	if err != nil || len(records) == 0 || records[0].DocumentID != refundID || records[0].DocumentName != "退款政策.txt" {
		t.Fatalf("records=%+v err=%v", records, err)
	}
	if records[0].LexicalRank == 0 || records[0].VectorRank == 0 || records[0].Score <= 0 || records[0].SegmentBatchID == "" || records[0].Position == 0 {
		t.Fatalf("first=%+v", records[0])
	}
	if len(records) > base.RetrievalCount {
		t.Fatalf("count=%d limit=%d", len(records), base.RetrievalCount)
	}
	if probe.reranked != 0 || records[0].Score >= 1 {
		t.Fatalf("reranked=%d first=%+v", probe.reranked, records[0])
	}

	// 多知识库来源经统一融合返回，并可按游标读取相邻分段。
	sources, err := service.Sources(ctx, identity.Organization.ID, []string{base.ID})
	if err != nil {
		t.Fatal(err)
	}
	result, err := knowledgeretrieval.Search(ctx, sources, knowledgeretrieval.Request{Queries: []string{"退款", "退款金额"}})
	if err != nil || len(result.Records) == 0 || result.Records[0].KnowledgeBaseID != base.ID || result.Records[0].DocumentID != refundID {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	var cursor knowledgeretrieval.Cursor
	for _, record := range result.Records {
		if record.DocumentID == refundID && record.Position == 1 {
			cursor = record.Cursor
		}
	}
	window, err := knowledgeretrieval.Search(ctx, sources, knowledgeretrieval.Request{Cursor: &cursor, After: 1})
	if err != nil || len(window.Records) != 2 || window.Records[0].SegmentID != cursor.SegmentID || window.Records[1].Position != 2 {
		t.Fatalf("cursor=%+v window=%+v err=%v", cursor, window, err)
	}

	// 向量路失败时保留词法路结果。
	probe.embedFail = true
	records, err = service.Retrieve(ctx, identity, base.ID, "退款")
	if err != nil || len(records) == 0 || records[0].DocumentID != refundID || records[0].VectorRank != 0 || records[0].LexicalRank != 1 {
		t.Fatalf("records=%+v err=%v", records, err)
	}
	// 没有词法词元时向量路失败即整体失败。
	if _, err := service.Retrieve(ctx, identity, base.ID, "什么"); err == nil {
		t.Fatal("expected embedding failure")
	}
	probe.embedFail = false

	// 开启重排后按重排模型返回的相关性重新排序。
	input := newKnowledgeBaseInput(t, db, identity, base.Name, base.Category)
	input.EmbeddingProviderID, input.RetrievalCount = base.EmbeddingProviderID, 2
	input.RerankProviderID, input.RerankModelIdentifier = base.EmbeddingProviderID, "rerank"
	if _, err := knowledgeaction.NewUpdateKnowledgeBaseAction(db).Execute(ctx, identity, base.ID, input); err != nil {
		t.Fatal(err)
	}
	records, err = service.Retrieve(ctx, identity, base.ID, "如何申请退款")
	if err != nil || probe.reranked != 1 || len(records) != 2 || records[0].DocumentID == refundID || records[0].Score != 1 {
		t.Fatalf("records=%+v reranked=%d err=%v", records, probe.reranked, err)
	}

	if _, err := service.Retrieve(ctx, identity, base.ID, "   "); !errors.Is(err, knowledgeaction.ErrRetrievalQueryInvalid) {
		t.Fatalf("err=%v", err)
	}
	other, _ := newDocumentFixture(t, db)
	if _, err := service.Retrieve(ctx, other.Identity, base.ID, "退款"); !errors.Is(err, knowledgeaction.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}

	// 文档删除后旧游标不可读取。
	if err := knowledgeaction.NewDeleteDocumentAction(db).Execute(ctx, identity, base.ID, refundID); err != nil {
		t.Fatal(err)
	}
	if _, err := knowledgeretrieval.Search(ctx, sources, knowledgeretrieval.Request{Cursor: &cursor}); !errors.Is(err, knowledgeaction.ErrSegmentStale) {
		t.Fatalf("err=%v", err)
	}
}
