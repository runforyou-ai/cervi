//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"

	knowledgeaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeprocessing"
	filecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// ListKnowledgeDocuments 返回当前企业分组中的文档。
func (o *directOperations) ListKnowledgeDocuments(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID string, input KnowledgeDocumentListInput) (KnowledgeDocumentList, error) {
	result, err := o.documentQuery.List(ctx, identity, baseID, knowledgeaction.DocumentListInput{GroupID: input.GroupID, Keyword: input.Keyword, Page: input.Page, PageSize: input.PageSize})
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
	records, err := o.createDocuments.Execute(ctx, identity, baseID, input.GroupID, input.FileIDs)
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

// MoveKnowledgeDocument 修改文档的分组归属。
func (o *directOperations) MoveKnowledgeDocument(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID, documentID string, input KnowledgeDocumentMoveInput) error {
	if err := o.moveDocument.Execute(ctx, identity, baseID, documentID, input.GroupID); err != nil {
		return o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentSaveFailed, identity.Organization.ID, baseID)
	}
	slog.Info("知识文档已移动", "knowledge_base_id", baseID, "document_id", documentID, "group_id", input.GroupID)
	return nil
}

// DeleteKnowledgeDocument 删除文档并安排原件清理。
func (o *directOperations) DeleteKnowledgeDocument(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID, documentID string) error {
	if err := o.deleteDocument.Execute(ctx, identity, baseID, documentID); err != nil {
		return o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentDeleteFailed, identity.Organization.ID, baseID)
	}
	slog.Info("知识文档已删除", "knowledge_base_id", baseID, "document_id", documentID)
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
	setting, err := o.getS3Setting.ExecuteForOrganization(ctx, record.OrganizationID)
	if err != nil {
		return KnowledgeDocumentPreviewRequest{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentReadFailed)
	}
	request, err := filecontent.PresignDownload(ctx, s3FileConfig(setting), record.StorageKey, "inline")
	if err != nil {
		return KnowledgeDocumentPreviewRequest{}, o.fileOperationError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentReadFailed)
	}
	return KnowledgeDocumentPreviewRequest{URL: request.URL, Headers: map[string]string{}}, nil
}

// knowledgeDocumentFromAction 转换本地文档元数据。
func knowledgeDocumentFromAction(meta RequestMeta, record knowledgeaction.DocumentRecord) KnowledgeDocument {
	message := ""
	if record.FailureCode != "" {
		key := cervii18n.ErrorKnowledgeProcessingFailed
		switch record.FailureCode {
		case "unavailable":
			key = cervii18n.ErrorKnowledgeProcessingUnavailable
		case "connection_timeout":
			key = cervii18n.ErrorKnowledgeConnectionTimeout
		case "request_timeout":
			key = cervii18n.ErrorKnowledgeRequestTimeout
		case "file_read_failed":
			key = cervii18n.ErrorKnowledgeOriginalReadFailed
		case "empty_content":
			key = cervii18n.ErrorKnowledgeContentEmpty
		case "recognition_required":
			key = cervii18n.ErrorKnowledgeRecognitionRequired
		case "encrypted_file":
			key = cervii18n.ErrorKnowledgeFileEncrypted
		case "parse_failed", "unsupported_file":
			key = cervii18n.ErrorKnowledgeParseFailed
		case "embedding_model_unavailable":
			key = cervii18n.ErrorKnowledgeEmbeddingUnavailable
		case "embedding_failed":
			key = cervii18n.ErrorKnowledgeEmbeddingFailed
		case "embedding_dimension_mismatch":
			key = cervii18n.ErrorKnowledgeEmbeddingDimension
		}
		message, _ = cervii18n.Localize(string(meta.Locale), key)
	}
	// 将文档处理阶段映射为展示状态。
	status := KnowledgeDocumentRunning
	switch record.Status {
	case domain.KnowledgeDocumentInitial:
		status = KnowledgeDocumentInitial
	case domain.KnowledgeDocumentQueued:
		status = KnowledgeDocumentQueued
	case domain.KnowledgeDocumentSucceeded:
		status = KnowledgeDocumentSucceeded
	case domain.KnowledgeDocumentFailed:
		status = KnowledgeDocumentFailed
	case domain.KnowledgeDocumentCancelled:
		status = KnowledgeDocumentCancelled
	}
	return KnowledgeDocument{ProcessingStatus: KnowledgeDocumentProcessingStatus(record.Status), SegmentBatchID: record.SegmentBatchID, SegmentCount: record.SegmentCount, FailureMessage: message, Format: KnowledgeDocumentFormat(strings.ToLower(filepath.Ext(record.Name))), ID: record.ID, GroupID: record.GroupID, Name: record.Name, ContentType: record.ContentType, ByteSize: record.ByteSize, Status: status, CreatedAt: record.CreatedAt}
}

// RetryKnowledgeDocument 按当前配置为文档安排新的处理任务。
func (o *directOperations) RetryKnowledgeDocument(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID, documentID string) error {
	if err := o.documentProcessing.Retry(ctx, identity, baseID, documentID, o.knowledgeProcessor.CheckConnection); err != nil {
		var failure *knowledgeprocessing.Error
		if errors.As(err, &failure) {
			key := cervii18n.ErrorKnowledgeProcessingUnavailable
			if failure.Code == "connection_timeout" {
				key = cervii18n.ErrorKnowledgeConnectionTimeout
			}
			return FailedError(meta, key)
		}
		return o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentRetryFailed, identity.Organization.ID, baseID)
	}
	slog.Info("知识文档已提交重试", "knowledge_base_id", baseID, "document_id", documentID)
	return nil
}

// ListKnowledgeDocumentSegments 返回可连续阅读的一页分段。
func (o *directOperations) ListKnowledgeDocumentSegments(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, baseID, documentID string, input KnowledgeDocumentSegmentInput) (KnowledgeDocumentSegmentPage, error) {
	page, err := o.documentQuery.Segments(ctx, identity, baseID, documentID, knowledgeprocessing.ListInput{Page: input.Page, PageSize: input.PageSize, SegmentBatchID: input.SegmentBatchID, AnchorSegmentID: input.AnchorSegmentID})
	if err != nil {
		return KnowledgeDocumentSegmentPage{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeDocumentReadFailed, identity.Organization.ID, baseID)
	}
	output := KnowledgeDocumentSegmentPage{SegmentBatchID: page.SegmentBatchID, Page: PageInfo{Number: page.Page, Size: page.PageSize, Total: page.Total}, AnchorSegmentID: page.AnchorSegmentID, AnchorPosition: page.AnchorPosition, Segments: make([]KnowledgeDocumentSegment, 0, len(page.Segments))}
	for _, segment := range page.Segments {
		output.Segments = append(output.Segments, KnowledgeDocumentSegment{ID: segment.ID, Position: segment.Position, Content: segment.Content, CharacterCount: segment.CharacterCount, PageNumber: segment.PageNumber, SourceLabel: segment.SourceLabel})
	}
	return output, nil
}
