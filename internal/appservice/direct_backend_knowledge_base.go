//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	knowledgebaseaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

// ListKnowledgeBases 返回当前企业的知识库列表。
func (b *DirectBackend) ListKnowledgeBases(ctx context.Context, meta RequestMeta) (KnowledgeBaseList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return KnowledgeBaseList{}, err
	}
	records, err := b.listKnowledgeBases.Execute(ctx, identity)
	if err != nil {
		return KnowledgeBaseList{}, b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseListFailed, identity.Organization.ID, "")
	}
	knowledgeBases := make([]KnowledgeBase, 0, len(records))
	for _, record := range records {
		knowledgeBases = append(knowledgeBases, knowledgeBaseFromAction(record))
	}
	return KnowledgeBaseList{KnowledgeBases: knowledgeBases}, nil
}

// ListExternalKnowledgeBaseOptions 返回指定连接可访问的外部知识库选项。
func (b *DirectBackend) ListExternalKnowledgeBaseOptions(
	ctx context.Context,
	meta RequestMeta,
	connectionID string,
) (ExternalKnowledgeBaseOptionList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return ExternalKnowledgeBaseOptionList{}, err
	}
	records, err := b.listExternalKnowledgeBaseOptions.Execute(ctx, identity, connectionID)
	if err != nil {
		if ctx.Err() != nil {
			return ExternalKnowledgeBaseOptionList{}, ctx.Err()
		}
		if stage, kind, classified := connectiontest.Details(err); classified {
			slog.Warn("Dify 知识库列表读取失败",
				"organization_id", identity.Organization.ID,
				"connection_id", connectionID,
				"stage", stage,
				"kind", kind,
			)
			return ExternalKnowledgeBaseOptionList{}, integrationConnectionRemoteError(meta, err)
		}
		return ExternalKnowledgeBaseOptionList{}, b.knowledgeBaseError(
			ctx, meta, err, cervii18n.ErrorKnowledgeBaseListFailed, identity.Organization.ID, "",
		)
	}
	knowledgeBases := make([]ExternalKnowledgeBaseOption, 0, len(records))
	for _, record := range records {
		knowledgeBases = append(knowledgeBases, ExternalKnowledgeBaseOption{
			ID: record.ID, Name: record.Name, Category: KnowledgeBaseCategory(record.Category),
		})
	}
	slog.Info("Dify 知识库列表读取成功",
		"organization_id", identity.Organization.ID,
		"connection_id", connectionID,
		"knowledge_base_count", len(knowledgeBases),
	)
	return ExternalKnowledgeBaseOptionList{KnowledgeBases: knowledgeBases}, nil
}

// ListKnowledgeDocuments 返回指定外部知识库的文档列表。
func (b *DirectBackend) ListKnowledgeDocuments(
	ctx context.Context,
	meta RequestMeta,
	knowledgeBaseID string,
	input KnowledgeDocumentListInput,
) (KnowledgeDocumentList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return KnowledgeDocumentList{}, err
	}
	output, err := b.listKnowledgeDocuments.Execute(ctx, identity, knowledgeBaseID, knowledgebaseaction.DocumentListInput{
		Keyword: input.Keyword, Status: optionalDomain[KnowledgeDocumentStatus, domain.KnowledgeDocumentStatus](input.Status),
		Page: input.Page, PageSize: input.PageSize,
	})
	if err != nil {
		return KnowledgeDocumentList{}, b.knowledgeDocumentReadError(
			ctx, meta, err, cervii18n.ErrorKnowledgeDocumentListFailed,
			identity.Organization.ID, knowledgeBaseID, "",
		)
	}
	documents := make([]KnowledgeDocumentSummary, 0, len(output.Documents))
	for _, document := range output.Documents {
		documents = append(documents, KnowledgeDocumentSummary{
			ID: document.ID, Name: document.Name, Status: KnowledgeDocumentStatus(document.Status),
			CreatedAt: document.CreatedAt,
		})
	}
	slog.Info("Dify 知识文档列表读取成功",
		"organization_id", identity.Organization.ID,
		"knowledge_base_id", knowledgeBaseID,
		"page", output.Page,
		"page_size", output.PageSize,
		"page_document_count", len(documents),
		"total_document_count", output.Total,
		"keyword_filtered", strings.TrimSpace(input.Keyword) != "",
		"status", optionalDomain[KnowledgeDocumentStatus, string](input.Status),
	)
	return KnowledgeDocumentList{
		Documents: documents,
		Page:      PageInfo{Number: output.Page, Size: output.PageSize, Total: output.Total},
	}, nil
}

// GetKnowledgeDocument 返回指定外部知识文档详情。
func (b *DirectBackend) GetKnowledgeDocument(
	ctx context.Context,
	meta RequestMeta,
	knowledgeBaseID, documentID string,
) (KnowledgeDocument, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return KnowledgeDocument{}, err
	}
	record, err := b.getKnowledgeDocument.Execute(ctx, identity, knowledgeBaseID, documentID)
	if err != nil {
		return KnowledgeDocument{}, b.knowledgeDocumentReadError(
			ctx, meta, err, cervii18n.ErrorKnowledgeDocumentReadFailed,
			identity.Organization.ID, knowledgeBaseID, documentID,
		)
	}
	slog.Info("Dify 知识文档读取成功",
		"organization_id", identity.Organization.ID,
		"knowledge_base_id", knowledgeBaseID,
		"document_id", documentID,
	)
	return KnowledgeDocument{
		ID: record.ID, Name: record.Name, Status: KnowledgeDocumentStatus(record.Status),
		WordCount: record.WordCount, HitCount: record.HitCount, CreatedAt: record.CreatedAt,
	}, nil
}

// ListKnowledgeDocumentSegments 返回指定外部知识文档的分段列表。
func (b *DirectBackend) ListKnowledgeDocumentSegments(
	ctx context.Context,
	meta RequestMeta,
	knowledgeBaseID, documentID string,
	input KnowledgeDocumentSegmentListInput,
) (KnowledgeDocumentSegmentList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return KnowledgeDocumentSegmentList{}, err
	}
	output, err := b.listKnowledgeDocumentSegments.Execute(
		ctx,
		identity,
		knowledgeBaseID,
		documentID,
		knowledgebaseaction.DocumentSegmentListInput{
			Keyword: input.Keyword, Page: input.Page, PageSize: input.PageSize,
		},
	)
	if err != nil {
		return KnowledgeDocumentSegmentList{}, b.knowledgeDocumentReadError(
			ctx, meta, err, cervii18n.ErrorKnowledgeDocumentSegmentListFailed,
			identity.Organization.ID, knowledgeBaseID, documentID,
		)
	}
	segments := make([]KnowledgeDocumentSegment, 0, len(output.Segments))
	for _, segment := range output.Segments {
		segments = append(segments, KnowledgeDocumentSegment{
			ID: segment.ID, Position: segment.Position, Content: segment.Content, Answer: segment.Answer,
			WordCount: segment.WordCount, HitCount: segment.HitCount,
			IndexStatus: KnowledgeDocumentSegmentIndexStatus(segment.IndexStatus),
			CreatedAt:   segment.CreatedAt,
		})
	}
	slog.Info("Dify 知识文档分段列表读取成功",
		"organization_id", identity.Organization.ID,
		"knowledge_base_id", knowledgeBaseID,
		"document_id", documentID,
		"page", output.Page,
		"page_size", output.PageSize,
		"page_segment_count", len(segments),
		"total_segment_count", output.Total,
		"keyword_filtered", strings.TrimSpace(input.Keyword) != "",
	)
	return KnowledgeDocumentSegmentList{
		Segments: segments,
		Page:     PageInfo{Number: output.Page, Size: output.PageSize, Total: output.Total},
	}, nil
}

// GetKnowledgeBase 返回当前企业中的知识库详情。
func (b *DirectBackend) GetKnowledgeBase(ctx context.Context, meta RequestMeta, knowledgeBaseID string) (KnowledgeBase, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return KnowledgeBase{}, err
	}
	record, err := b.getKnowledgeBase.Execute(ctx, identity, knowledgeBaseID)
	if err != nil {
		return KnowledgeBase{}, b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseReadFailed, identity.Organization.ID, knowledgeBaseID)
	}
	return knowledgeBaseFromAction(*record), nil
}

// CreateKnowledgeBase 创建企业知识库。
func (b *DirectBackend) CreateKnowledgeBase(ctx context.Context, meta RequestMeta, input KnowledgeBaseInput) (KnowledgeBase, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return KnowledgeBase{}, err
	}
	record, err := b.createKnowledgeBase.Execute(ctx, identity, knowledgebaseaction.Input{
		Name: input.Name, Category: domain.KnowledgeBaseCategory(input.Category), Description: input.Description,
		IntegrationConnectionID: input.IntegrationConnectionID, ExternalResourceID: input.ExternalResourceID,
	})
	if err != nil {
		return KnowledgeBase{}, b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseCreateFailed, identity.Organization.ID, "")
	}
	slog.Info("知识库创建成功", "organization_id", identity.Organization.ID, "knowledge_base_id", record.ID, "category", record.Category, "external", record.IntegrationConnectionID != "")
	return knowledgeBaseFromAction(*record), nil
}

// UpdateKnowledgeBase 修改企业知识库。
func (b *DirectBackend) UpdateKnowledgeBase(ctx context.Context, meta RequestMeta, knowledgeBaseID string, input KnowledgeBaseInput) (KnowledgeBase, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return KnowledgeBase{}, err
	}
	record, err := b.updateKnowledgeBase.Execute(ctx, identity, knowledgeBaseID, knowledgebaseaction.Input{
		Name: input.Name, Category: domain.KnowledgeBaseCategory(input.Category), Description: input.Description,
		IntegrationConnectionID: input.IntegrationConnectionID, ExternalResourceID: input.ExternalResourceID,
	})
	if err != nil {
		return KnowledgeBase{}, b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseUpdateFailed, identity.Organization.ID, knowledgeBaseID)
	}
	slog.Info("知识库保存成功", "organization_id", identity.Organization.ID, "knowledge_base_id", record.ID, "category", record.Category, "external", record.IntegrationConnectionID != "")
	return knowledgeBaseFromAction(*record), nil
}

// DeleteKnowledgeBase 删除企业知识库。
func (b *DirectBackend) DeleteKnowledgeBase(ctx context.Context, meta RequestMeta, knowledgeBaseID string) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.deleteKnowledgeBase.Execute(ctx, identity, knowledgeBaseID); err != nil {
		return b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseDeleteFailed, identity.Organization.ID, knowledgeBaseID)
	}
	slog.Info("知识库删除成功", "organization_id", identity.Organization.ID, "knowledge_base_id", knowledgeBaseID)
	return nil
}

// CreateKnowledgeGroup 创建知识库分组。
func (b *DirectBackend) CreateKnowledgeGroup(ctx context.Context, meta RequestMeta, knowledgeBaseID string, input KnowledgeGroupInput) (KnowledgeBase, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return KnowledgeBase{}, err
	}
	record, err := b.createKnowledgeGroup.Execute(ctx, identity, knowledgeBaseID, knowledgebaseaction.GroupInput{Name: input.Name, ParentID: input.ParentID})
	if err != nil {
		return KnowledgeBase{}, b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeGroupCreateFailed, identity.Organization.ID, knowledgeBaseID)
	}
	slog.Info("知识库分组创建成功", "organization_id", identity.Organization.ID, "knowledge_base_id", knowledgeBaseID, "parent_group_id", input.ParentID)
	return knowledgeBaseFromAction(*record), nil
}

// UpdateKnowledgeGroup 修改知识库分组。
func (b *DirectBackend) UpdateKnowledgeGroup(ctx context.Context, meta RequestMeta, knowledgeBaseID, groupID string, input KnowledgeGroupInput) (KnowledgeBase, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return KnowledgeBase{}, err
	}
	record, err := b.updateKnowledgeGroup.Execute(ctx, identity, knowledgeBaseID, groupID, knowledgebaseaction.GroupInput{Name: input.Name, ParentID: input.ParentID})
	if err != nil {
		return KnowledgeBase{}, b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeGroupUpdateFailed, identity.Organization.ID, knowledgeBaseID)
	}
	slog.Info("知识库分组保存成功", "organization_id", identity.Organization.ID, "knowledge_base_id", knowledgeBaseID, "group_id", groupID)
	return knowledgeBaseFromAction(*record), nil
}

// DeleteKnowledgeGroup 删除不含子分组的知识库分组。
func (b *DirectBackend) DeleteKnowledgeGroup(ctx context.Context, meta RequestMeta, knowledgeBaseID, groupID string) (KnowledgeBase, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return KnowledgeBase{}, err
	}
	record, err := b.deleteKnowledgeGroup.Execute(ctx, identity, knowledgeBaseID, groupID)
	if err != nil {
		return KnowledgeBase{}, b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeGroupDeleteFailed, identity.Organization.ID, knowledgeBaseID)
	}
	slog.Info("知识库分组删除成功", "organization_id", identity.Organization.ID, "knowledge_base_id", knowledgeBaseID, "group_id", groupID)
	return knowledgeBaseFromAction(*record), nil
}

// knowledgeBaseError 转换知识库领域错误。
func (b *DirectBackend) knowledgeBaseError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID, knowledgeBaseID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var validationError *common.FieldError
	if errors.As(err, &validationError) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, knowledgeBaseFieldKeys(validationError.Fields))
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, knowledgebaseaction.ErrNotFound) {
		return NotFoundError(meta, cervii18n.ErrorKnowledgeBaseNotFound)
	}
	if errors.Is(err, knowledgebaseaction.ErrGroupNotFound) {
		return NotFoundError(meta, cervii18n.ErrorKnowledgeGroupNotFound)
	}
	if errors.Is(err, knowledgebaseaction.ErrGroupInvalid) {
		return InvalidError(meta, cervii18n.ErrorKnowledgeGroupInvalid, nil)
	}
	if errors.Is(err, knowledgebaseaction.ErrGroupNotEmpty) {
		return InvalidError(meta, cervii18n.ErrorKnowledgeGroupNotEmpty, nil)
	}
	if errors.Is(err, knowledgebaseaction.ErrExternalGroupUnsupported) {
		return InvalidError(meta, cervii18n.ErrorKnowledgeGroupExternalUnsupported, nil)
	}
	if errors.Is(err, knowledgebaseaction.ErrDocumentNotFound) {
		return NotFoundError(meta, cervii18n.ErrorKnowledgeDocumentNotFound)
	}
	if errors.Is(err, knowledgebaseaction.ErrDocumentReadUnsupported) {
		return InvalidError(meta, cervii18n.ErrorKnowledgeDocumentReadUnsupported, nil)
	}
	attributes := []any{"organization_id", organizationID, "failure", failureKey, "error", err}
	if knowledgeBaseID != "" {
		attributes = append(attributes, "knowledge_base_id", knowledgeBaseID)
	}
	slog.Warn("知识库操作失败", attributes...)
	return FailedError(meta, failureKey)
}

// knowledgeDocumentReadError 转换知识文档远程读取错误并保留本地知识库错误语义。
func (b *DirectBackend) knowledgeDocumentReadError(
	ctx context.Context,
	meta RequestMeta,
	err error,
	failureKey cervii18n.Key,
	organizationID, knowledgeBaseID, documentID string,
) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if stage, kind, classified := connectiontest.Details(err); classified {
		attributes := []any{
			"organization_id", organizationID,
			"knowledge_base_id", knowledgeBaseID,
			"stage", stage,
			"kind", kind,
		}
		if documentID != "" {
			attributes = append(attributes, "document_id", documentID)
		}
		slog.Warn("Dify 知识文档读取失败", attributes...)
		if kind == connectiontest.FailureNotFound && documentID != "" {
			return NotFoundError(meta, cervii18n.ErrorKnowledgeDocumentNotFound)
		}
		switch kind {
		case connectiontest.FailureUnauthorized,
			connectiontest.FailureForbidden,
			connectiontest.FailureRateLimited,
			connectiontest.FailureTimeout,
			connectiontest.FailureNetwork,
			connectiontest.FailureTLS,
			connectiontest.FailureUnavailable:
			return integrationConnectionRemoteError(meta, err)
		default:
			return FailedError(meta, failureKey)
		}
	}
	return b.knowledgeBaseError(ctx, meta, err, failureKey, organizationID, knowledgeBaseID)
}

// knowledgeBaseFromAction 转换知识库契约。
func knowledgeBaseFromAction(record knowledgebaseaction.Record) KnowledgeBase {
	return KnowledgeBase{
		ID: record.ID, Name: record.Name, Category: KnowledgeBaseCategory(record.Category), Description: record.Description,
		IntegrationConnectionID: record.IntegrationConnectionID, ExternalResourceID: record.ExternalResourceID,
		Groups:    knowledgeGroupsFromAction(record.Groups),
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
	}
}

// knowledgeGroupsFromAction 转换知识库分组树契约。
func knowledgeGroupsFromAction(records []knowledgebaseaction.GroupRecord) []KnowledgeGroup {
	groups := make([]KnowledgeGroup, 0, len(records))
	for _, record := range records {
		parentID := ""
		if record.ParentID != nil {
			parentID = *record.ParentID
		}
		groups = append(groups, KnowledgeGroup{
			ID: record.ID, ParentID: parentID, Name: record.Name, IsDefault: record.IsDefault,
			Children: knowledgeGroupsFromAction(record.Children),
		})
	}
	return groups
}

// knowledgeBaseFieldKeys 把知识库校验错误码映射为本地化文案键。
func knowledgeBaseFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		knowledgebaseaction.ValidationNameRequired:                 cervii18n.FieldKnowledgeBaseNameRequired,
		knowledgebaseaction.ValidationNameTooLong:                  cervii18n.FieldKnowledgeBaseNameTooLong,
		knowledgebaseaction.ValidationNameDuplicate:                cervii18n.FieldKnowledgeBaseNameDuplicate,
		knowledgebaseaction.ValidationCategoryInvalid:              cervii18n.FieldKnowledgeBaseCategoryInvalid,
		knowledgebaseaction.ValidationDescriptionTooLong:           cervii18n.FieldKnowledgeBaseDescriptionTooLong,
		knowledgebaseaction.ValidationIntegrationConnectionInvalid: cervii18n.FieldKnowledgeBaseIntegrationConnectionInvalid,
		knowledgebaseaction.ValidationExternalResourceRequired:     cervii18n.FieldKnowledgeBaseExternalResourceRequired,
		knowledgebaseaction.ValidationExternalResourceTooLong:      cervii18n.FieldKnowledgeBaseExternalResourceTooLong,
		knowledgebaseaction.ValidationExternalResourceDuplicate:    cervii18n.FieldKnowledgeBaseExternalResourceDuplicate,
		knowledgebaseaction.ValidationGroupNameRequired:            cervii18n.FieldKnowledgeGroupNameRequired,
		knowledgebaseaction.ValidationGroupNameTooLong:             cervii18n.FieldKnowledgeGroupNameTooLong,
		knowledgebaseaction.ValidationGroupNameDuplicate:           cervii18n.FieldKnowledgeGroupNameDuplicate,
		knowledgebaseaction.ValidationGroupParentInvalid:           cervii18n.FieldKnowledgeGroupParentInvalid,
		knowledgebaseaction.ValidationDocumentQueryInvalid:         cervii18n.FieldKnowledgeDocumentQueryInvalid,
	}
	return translateValidationFields(fields, keys)
}
