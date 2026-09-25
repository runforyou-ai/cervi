//go:build server

package appservice

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	knowledgeaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	filecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// ListKnowledgeDocuments 返回当前企业知识库中的文档。
func (o *directOperations) ListKnowledgeDocuments(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID string, input KnowledgeDocumentListInput) (KnowledgeDocumentList, error) {
	result, err := o.documentQuery.List(ctx, identity, baseID, knowledgeaction.DocumentListInput{Keyword: input.Keyword, Page: input.Page, PageSize: input.PageSize})
	if err != nil {
		return KnowledgeDocumentList{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentReadFailed, identity.Organization.ID, baseID)
	}
	output := KnowledgeDocumentList{Documents: make([]KnowledgeDocument, 0, len(result.Documents)), Page: PageInfo{Number: result.Page, Size: result.PageSize, Total: result.Total}}
	for _, record := range result.Documents {
		output.Documents = append(output.Documents, knowledgeDocumentFromAction(meta, record))
	}
	return output, nil
}

// GetKnowledgeDocument 返回文档详情。
func (o *directOperations) GetKnowledgeDocument(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID, documentID string) (KnowledgeDocument, error) {
	record, err := o.documentQuery.Get(ctx, identity, baseID, documentID)
	if err != nil {
		return KnowledgeDocument{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentReadFailed, identity.Organization.ID, baseID)
	}
	return knowledgeDocumentFromAction(meta, *record), nil
}

// CreateKnowledgeDocuments 在事务中创建文档并激活已上传原件。
func (o *directOperations) CreateKnowledgeDocuments(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID string, input KnowledgeDocumentBatchInput) (KnowledgeDocumentBatch, error) {
	records, err := o.createDocuments.Execute(ctx, identity, baseID, input.FileIDs)
	if err != nil {
		return KnowledgeDocumentBatch{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentSaveFailed, identity.Organization.ID, baseID)
	}
	output := KnowledgeDocumentBatch{Documents: make([]KnowledgeDocument, 0, len(records))}
	for _, record := range records {
		output.Documents = append(output.Documents, knowledgeDocumentFromAction(meta, record))
	}
	slog.Info("知识文档已保存", "organization_id", identity.Organization.ID, "knowledge_base_id", baseID, "document_count", len(records))
	return output, nil
}

// DeleteKnowledgeDocument 删除文档并安排原件清理。
func (o *directOperations) DeleteKnowledgeDocument(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID, documentID string) error {
	if err := o.deleteDocument.Execute(ctx, identity, baseID, documentID); err != nil {
		return o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentDeleteFailed, identity.Organization.ID, baseID)
	}
	slog.Info("知识文档已删除", "knowledge_base_id", baseID, "document_id", documentID)
	return nil
}

// CreateKnowledgeTextDocument 创建在线编写的文档并安排索引。
func (o *directOperations) CreateKnowledgeTextDocument(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID string, input KnowledgeTextDocumentInput) (KnowledgeDocument, error) {
	record, err := o.saveTextDocument.Execute(ctx, identity, baseID, "", knowledgeaction.TextDocumentInput{Title: input.Title, Content: input.Content})
	if err != nil {
		return KnowledgeDocument{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentSaveFailed, identity.Organization.ID, baseID)
	}
	slog.Info("在线文档已创建", "knowledge_base_id", baseID, "document_id", record.ID)
	return knowledgeDocumentFromAction(meta, *record), nil
}

// GetKnowledgeDocumentContent 返回在线文档正文或网页抓取快照。
func (o *directOperations) GetKnowledgeDocumentContent(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID, documentID string) (KnowledgeDocumentContent, error) {
	record, err := o.documentQuery.Content(ctx, identity, baseID, documentID)
	if err != nil {
		return KnowledgeDocumentContent{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentReadFailed, identity.Organization.ID, baseID)
	}
	return KnowledgeDocumentContent{Document: knowledgeDocumentFromAction(meta, record.Document), Content: record.Content}, nil
}

// UpdateKnowledgeDocumentContent 修改在线文档的名称与正文并安排索引。
func (o *directOperations) UpdateKnowledgeDocumentContent(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID, documentID string, input KnowledgeDocumentContentInput) (KnowledgeDocument, error) {
	record, err := o.saveTextDocument.Execute(ctx, identity, baseID, documentID, knowledgeaction.TextDocumentInput{Title: input.Title, Content: input.Content})
	if err != nil {
		return KnowledgeDocument{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentSaveFailed, identity.Organization.ID, baseID)
	}
	slog.Info("在线文档已保存", "knowledge_base_id", baseID, "document_id", documentID)
	return knowledgeDocumentFromAction(meta, *record), nil
}

// RenameKnowledgeDocument 修改在线文档或网页文档的名称。
func (o *directOperations) RenameKnowledgeDocument(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID, documentID string, input KnowledgeDocumentRenameInput) (KnowledgeDocument, error) {
	record, err := o.renameDocument.Execute(ctx, identity, baseID, documentID, input.Title)
	if err != nil {
		return KnowledgeDocument{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentSaveFailed, identity.Organization.ID, baseID)
	}
	slog.Info("知识文档已改名", "knowledge_base_id", baseID, "document_id", documentID)
	return knowledgeDocumentFromAction(meta, *record), nil
}

// CreateKnowledgeWebDocument 导入网页并安排首次抓取。
func (o *directOperations) CreateKnowledgeWebDocument(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID string, input KnowledgeWebDocumentInput) (KnowledgeDocument, error) {
	record, err := o.createWebDocument.Execute(ctx, identity, baseID, knowledgeaction.WebDocumentInput{Title: input.Title, SourceURL: input.SourceURL})
	if err != nil {
		return KnowledgeDocument{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentSaveFailed, identity.Organization.ID, baseID)
	}
	slog.Info("网页文档已导入", "knowledge_base_id", baseID, "document_id", record.ID, "source_url", record.SourceURL)
	return knowledgeDocumentFromAction(meta, *record), nil
}

// RefetchKnowledgeDocument 重新抓取网页文档并重新索引。
func (o *directOperations) RefetchKnowledgeDocument(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID, documentID string, input KnowledgeDocumentRefetchInput) error {
	if err := o.documentProcessing.Refetch(ctx, identity, baseID, documentID, input.SourceURL); err != nil {
		return o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentRetryFailed, identity.Organization.ID, baseID)
	}
	slog.Info("网页文档已提交重新抓取", "knowledge_base_id", baseID, "document_id", documentID)
	return nil
}

// GetKnowledgeDocumentPreview 按原件实际存储类型返回受控读取请求。
func (o *directOperations) GetKnowledgeDocumentPreview(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID, documentID string) (KnowledgeDocumentPreviewRequest, error) {
	record, err := o.documentQuery.File(ctx, identity, baseID, documentID)
	if err != nil {
		return KnowledgeDocumentPreviewRequest{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentReadFailed, identity.Organization.ID, baseID)
	}
	if record.StorageBackend == string(domain.FileStorageBackendLocal) {
		url, err := fileContentURL(domain.FileStorageBackendLocal, record.StorageKey, "")
		if err != nil {
			return KnowledgeDocumentPreviewRequest{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentReadFailed)
		}
		return KnowledgeDocumentPreviewRequest{URL: url, Headers: map[string]string{"Authorization": "Bearer " + meta.Token}}, nil
	}
	request, err := filecontent.PresignDownload(ctx, o.s3, record.StorageKey, "inline")
	if err != nil {
		return KnowledgeDocumentPreviewRequest{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentReadFailed)
	}
	return KnowledgeDocumentPreviewRequest{URL: request.URL, Headers: map[string]string{}}, nil
}

// knowledgeDocumentFromAction 转换本地文档元数据。
func knowledgeDocumentFromAction(meta RequestMeta, record knowledgeaction.DocumentRecord) KnowledgeDocument {
	status, message := knowledgeIndexPresentation(meta, record.Status, record.FailureCode)
	// 在线文档与网页文档的正文是 Markdown，名称不带扩展名。
	format := KnowledgeDocumentMD
	if record.SourceKind == domain.KnowledgeDocumentSourceFile {
		format = KnowledgeDocumentFormat(strings.ToLower(filepath.Ext(record.Name)))
	}
	return KnowledgeDocument{ProcessingStatus: KnowledgeIndexProcessingStatus(record.Status), SegmentBatchID: record.SegmentBatchID, SegmentCount: record.SegmentCount, FailureMessage: message, Format: format, SourceKind: KnowledgeDocumentSourceKind(record.SourceKind), ID: record.ID, Name: record.Name, SourceURL: record.SourceURL, ContentType: record.ContentType, ByteSize: record.ByteSize, Status: status, UpdatedAt: record.UpdatedAt}
}

// RetryKnowledgeDocument 按当前配置为文档安排新的处理任务。
func (o *directOperations) RetryKnowledgeDocument(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID, documentID string) error {
	if err := o.documentProcessing.Retry(ctx, identity, baseID, documentID); err != nil {
		return o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentRetryFailed, identity.Organization.ID, baseID)
	}
	slog.Info("知识文档已提交重试", "knowledge_base_id", baseID, "document_id", documentID)
	return nil
}

// ListKnowledgeDocumentSegments 返回可连续阅读的一页分段。
func (o *directOperations) ListKnowledgeDocumentSegments(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID, documentID string, input KnowledgeDocumentSegmentInput) (KnowledgeDocumentSegmentPage, error) {
	page, err := o.documentQuery.Segments(ctx, identity, baseID, documentID, knowledgeaction.SegmentQueryInput{Page: input.Page, PageSize: input.PageSize, SegmentBatchID: input.SegmentBatchID, AnchorSegmentID: input.AnchorSegmentID})
	if err != nil {
		return KnowledgeDocumentSegmentPage{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentReadFailed, identity.Organization.ID, baseID)
	}
	output := KnowledgeDocumentSegmentPage{SegmentBatchID: page.SegmentBatchID, Page: PageInfo{Number: page.Page, Size: page.PageSize, Total: page.Total}, AnchorSegmentID: page.AnchorSegmentID, AnchorPosition: page.AnchorPosition, Segments: make([]KnowledgeDocumentSegment, 0, len(page.Segments))}
	for _, segment := range page.Segments {
		output.Segments = append(output.Segments, KnowledgeDocumentSegment{ID: segment.ID, Position: segment.Position, Context: segment.Context, Content: segment.Content, CharacterCount: segment.CharacterCount})
	}
	return output, nil
}
