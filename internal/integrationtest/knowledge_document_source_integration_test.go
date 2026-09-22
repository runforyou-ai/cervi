//go:build server

package integrationtest

import (
	"context"
	"strings"
	"testing"

	knowledgeaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/domain"
	servertest "github.com/runforyou-ai/cervi/internal/servertest"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestKnowledgeDocumentSourceWithoutFile 验证没有原件的文档来源在列表、详情、关键词过滤和混合召回中按文档标题呈现。
func TestKnowledgeDocumentSourceWithoutFile(t *testing.T) {
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
	documentID := publishRetrievalDocument(t, db, probe, identity, base, "退款政策.txt", strings.Repeat("签收后七天内可以申请退款，退款金额原路返回。", 30))

	// 把已发布文档改写成在线编写来源，模拟没有原件的内容形态。
	content := "签收后七天内可以申请退款。"
	if _, err := db.NewUpdate().Model((*servermodels.KnowledgeDocument)(nil)).
		Set("source_kind = ?", domain.KnowledgeDocumentSourceText).Set("title = ?", "在线退款说明").Set("file_id = NULL").
		Where("id = ?", documentID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.NewInsert().Model(&servermodels.KnowledgeDocumentContent{DocumentID: documentID, Content: content}).Exec(ctx); err != nil {
		t.Fatal(err)
	}

	query := knowledgeaction.NewDocumentQuery(db)
	list, err := query.List(ctx, identity, base.ID, knowledgeaction.DocumentListInput{})
	if err != nil || len(list.Documents) != 1 {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	record := list.Documents[0]
	if record.SourceKind != domain.KnowledgeDocumentSourceText || record.Name != "在线退款说明" {
		t.Fatalf("record=%+v", record)
	}
	if record.ContentType != domain.KnowledgeDocumentMarkdownContentType || record.ByteSize != int64(len(content)) {
		t.Fatalf("content_type=%s byte_size=%d", record.ContentType, record.ByteSize)
	}
	detail, err := query.Get(ctx, identity, base.ID, documentID)
	if err != nil || detail.Name != "在线退款说明" {
		t.Fatalf("detail=%+v err=%v", detail, err)
	}

	// 关键词按文档标题匹配，原件文件名不再参与过滤。
	titled, err := query.List(ctx, identity, base.ID, knowledgeaction.DocumentListInput{Keyword: "在线退款"})
	if err != nil || len(titled.Documents) != 1 {
		t.Fatalf("titled=%+v err=%v", titled, err)
	}
	named, err := query.List(ctx, identity, base.ID, knowledgeaction.DocumentListInput{Keyword: "退款政策.txt"})
	if err != nil || len(named.Documents) != 0 {
		t.Fatalf("named=%+v err=%v", named, err)
	}

	// 已发布分段在原件缺失后仍然可召回，来源名称取文档标题。
	records, err := knowledgeaction.NewRetrievalService(db, probe, probe).Retrieve(ctx, identity, base.ID, "如何申请退款")
	if err != nil || len(records) == 0 {
		t.Fatalf("records=%+v err=%v", records, err)
	}
	if records[0].DocumentID != documentID || records[0].DocumentName != "在线退款说明" {
		t.Fatalf("first=%+v", records[0])
	}
}
