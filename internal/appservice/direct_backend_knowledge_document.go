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
)

// ListKnowledgeDocuments 返回当前企业分组中的文档。
func (b *DirectBackend) ListKnowledgeDocuments(ctx context.Context, meta RequestMeta, baseID string, input KnowledgeDocumentListInput) (KnowledgeDocumentList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return KnowledgeDocumentList{}, err
	}
	result, err := b.documentQuery.List(ctx, identity, baseID, knowledgeaction.DocumentListInput{GroupID: input.GroupID, Keyword: input.Keyword, Page: input.Page, PageSize: input.PageSize})
	if err != nil {
		return KnowledgeDocumentList{}, b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentReadFailed, identity.Organization.ID, baseID)
	}
	output := KnowledgeDocumentList{Documents: make([]KnowledgeDocument, 0, len(result.Documents)), Page: PageInfo{Number: result.Page, Size: result.PageSize, Total: result.Total}}
	for _, record := range result.Documents {
		output.Documents = append(output.Documents, knowledgeDocumentFromAction(record))
	}
	return output, nil
}

// GetKnowledgeDocument 返回文档详情。
func (b *DirectBackend) GetKnowledgeDocument(ctx context.Context, meta RequestMeta, baseID, documentID string) (KnowledgeDocument, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	record, err := b.documentQuery.Get(ctx, identity, baseID, documentID)
	if err != nil {
		return KnowledgeDocument{}, b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentReadFailed, identity.Organization.ID, baseID)
	}
	return knowledgeDocumentFromAction(*record), nil
}

// CreateKnowledgeDocuments 在事务中创建文档并激活已上传原件。
func (b *DirectBackend) CreateKnowledgeDocuments(ctx context.Context, meta RequestMeta, baseID string, input KnowledgeDocumentBatchInput) (KnowledgeDocumentBatch, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return KnowledgeDocumentBatch{}, err
	}
	records, err := b.createDocuments.Execute(ctx, identity, baseID, input.GroupID, input.FileIDs)
	if err != nil {
		return KnowledgeDocumentBatch{}, b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentSaveFailed, identity.Organization.ID, baseID)
	}
	output := KnowledgeDocumentBatch{Documents: make([]KnowledgeDocument, 0, len(records))}
	for _, record := range records {
		output.Documents = append(output.Documents, knowledgeDocumentFromAction(record))
	}
	slog.Info("知识文档已保存", "organization_id", identity.Organization.ID, "knowledge_base_id", baseID, "document_count", len(records))
	return output, nil
}

// MoveKnowledgeDocument 修改文档的分组归属。
func (b *DirectBackend) MoveKnowledgeDocument(ctx context.Context, meta RequestMeta, baseID, documentID string, input KnowledgeDocumentMoveInput) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.moveDocument.Execute(ctx, identity, baseID, documentID, input.GroupID); err != nil {
		return b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentSaveFailed, identity.Organization.ID, baseID)
	}
	slog.Info("知识文档已移动", "knowledge_base_id", baseID, "document_id", documentID, "group_id", input.GroupID)
	return nil
}

// DeleteKnowledgeDocument 删除文档并安排原件清理。
func (b *DirectBackend) DeleteKnowledgeDocument(ctx context.Context, meta RequestMeta, baseID, documentID string) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.deleteDocument.Execute(ctx, identity, baseID, documentID); err != nil {
		return b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentDeleteFailed, identity.Organization.ID, baseID)
	}
	slog.Info("知识文档已删除", "knowledge_base_id", baseID, "document_id", documentID)
	return nil
}

// GetKnowledgeDocumentPreview 按原件实际存储类型返回受控读取请求。
func (b *DirectBackend) GetKnowledgeDocumentPreview(ctx context.Context, meta RequestMeta, baseID, documentID string) (KnowledgeDocumentPreviewRequest, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return KnowledgeDocumentPreviewRequest{}, err
	}
	record, err := b.documentQuery.File(ctx, identity, baseID, documentID)
	if err != nil {
		return KnowledgeDocumentPreviewRequest{}, b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentReadFailed, identity.Organization.ID, baseID)
	}
	if record.StorageBackend == string(domain.FileStorageBackendLocal) {
		url, err := fileContentURL(domain.FileStorageBackendLocal, record.StorageKey, "")
		if err != nil {
			return KnowledgeDocumentPreviewRequest{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentReadFailed)
		}
		return KnowledgeDocumentPreviewRequest{URL: url, Headers: map[string]string{"Authorization": "Bearer " + meta.Token}}, nil
	}
	setting, err := b.getS3Setting.ExecuteForOrganization(ctx, record.OrganizationID)
	if err != nil {
		return KnowledgeDocumentPreviewRequest{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentReadFailed)
	}
	request, err := filecontent.PresignDownload(ctx, s3FileConfig(setting), record.StorageKey, "inline")
	if err != nil {
		return KnowledgeDocumentPreviewRequest{}, b.fileOperationError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentReadFailed)
	}
	return KnowledgeDocumentPreviewRequest{URL: request.URL, Headers: map[string]string{}}, nil
}

// knowledgeDocumentFromAction 转换本地文档元数据。
func knowledgeDocumentFromAction(record knowledgeaction.DocumentRecord) KnowledgeDocument {
	return KnowledgeDocument{Format: KnowledgeDocumentFormat(strings.ToLower(filepath.Ext(record.Name))), ID: record.ID, GroupID: record.GroupID, Name: record.Name, ContentType: record.ContentType, ByteSize: record.ByteSize, Status: KnowledgeDocumentStatus(record.Status), CreatedAt: record.CreatedAt}
}
