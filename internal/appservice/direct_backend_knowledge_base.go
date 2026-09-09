//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	knowledgebaseaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
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
		EmbeddingProviderID:      input.EmbeddingProviderID,
		EmbeddingModelIdentifier: input.EmbeddingModelIdentifier,
		EmbeddingDimension:       input.EmbeddingDimension,
		ChunkLength:              input.ChunkLength,
		ChunkOverlap:             input.ChunkOverlap,
		RetrievalCount:           input.RetrievalCount,
		RerankProviderID:         input.RerankProviderID,
		RerankModelIdentifier:    input.RerankModelIdentifier,
	})
	if err != nil {
		return KnowledgeBase{}, b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseCreateFailed, identity.Organization.ID, "")
	}
	slog.Info("知识库创建成功", "organization_id", identity.Organization.ID, "knowledge_base_id", record.ID, "category", record.Category)
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
		EmbeddingProviderID:      input.EmbeddingProviderID,
		EmbeddingModelIdentifier: input.EmbeddingModelIdentifier,
		EmbeddingDimension:       input.EmbeddingDimension,
		ChunkLength:              input.ChunkLength,
		ChunkOverlap:             input.ChunkOverlap,
		RetrievalCount:           input.RetrievalCount,
		RerankProviderID:         input.RerankProviderID,
		RerankModelIdentifier:    input.RerankModelIdentifier,
	})
	if err != nil {
		return KnowledgeBase{}, b.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseUpdateFailed, identity.Organization.ID, knowledgeBaseID)
	}
	slog.Info("知识库保存成功", "organization_id", identity.Organization.ID, "knowledge_base_id", record.ID, "category", record.Category)
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

// DeleteKnowledgeGroup 删除不含子分组和问答的知识库分组。
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
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, knowledgeBaseFieldKeys(validationError.Fields))
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, fileaction.ErrFileNotFound) {
		return NotFoundError(meta, cervii18n.ErrorFileNotFound)
	}
	if errors.Is(err, knowledgebaseaction.ErrDocumentNotFound) {
		return NotFoundError(meta, cervii18n.ErrorKnowledgeDocumentNotFound)
	}
	if errors.Is(err, knowledgebaseaction.ErrDocumentUnsupported) {
		return InvalidError(meta, cervii18n.ErrorKnowledgeDocumentUnsupported, nil)
	}
	if errors.Is(err, knowledgebaseaction.ErrDocumentBatchInvalid) {
		return InvalidError(meta, cervii18n.ErrorKnowledgeDocumentBatchInvalid, nil)
	}
	if errors.Is(err, knowledgebaseaction.ErrQANotFound) {
		return NotFoundError(meta, cervii18n.ErrorKnowledgeQANotFound)
	}
	if errors.Is(err, knowledgebaseaction.ErrQAUnsupported) {
		return InvalidError(meta, cervii18n.ErrorKnowledgeQAUnsupported, nil)
	}
	if errors.Is(err, knowledgebaseaction.ErrBaseHasContent) {
		return InvalidError(meta, cervii18n.ErrorKnowledgeBaseHasContent, nil)
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
	attributes := []any{"organization_id", organizationID, "failure", failureKey, "error", err}
	if knowledgeBaseID != "" {
		attributes = append(attributes, "knowledge_base_id", knowledgeBaseID)
	}
	slog.Warn("知识库操作失败", attributes...)
	return FailedError(meta, failureKey)
}

// knowledgeBaseFromAction 转换知识库契约。
func knowledgeBaseFromAction(record knowledgebaseaction.Record) KnowledgeBase {
	return KnowledgeBase{
		ID: record.ID, Name: record.Name, Category: KnowledgeBaseCategory(record.Category), Description: record.Description,
		Groups:                   knowledgeGroupsFromAction(record.Groups),
		EmbeddingProviderID:      record.EmbeddingProviderID,
		EmbeddingModelIdentifier: record.EmbeddingModelIdentifier,
		EmbeddingDimension:       record.EmbeddingDimension,
		ChunkLength:              record.ChunkLength,
		ChunkOverlap:             record.ChunkOverlap,
		RetrievalCount:           record.RetrievalCount,
		RerankProviderID:         record.RerankProviderID,
		RerankModelIdentifier:    record.RerankModelIdentifier,

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
		knowledgebaseaction.ValidationEmbeddingModelInvalid:     cervii18n.FieldKnowledgeBaseEmbeddingModelInvalid,
		knowledgebaseaction.ValidationEmbeddingDimensionInvalid: cervii18n.FieldKnowledgeBaseEmbeddingDimensionInvalid,
		knowledgebaseaction.ValidationChunkLengthInvalid:        cervii18n.FieldKnowledgeBaseChunkLengthInvalid,
		knowledgebaseaction.ValidationChunkOverlapInvalid:       cervii18n.FieldKnowledgeBaseChunkOverlapInvalid,
		knowledgebaseaction.ValidationRetrievalCountInvalid:     cervii18n.FieldKnowledgeBaseRetrievalCountInvalid,
		knowledgebaseaction.ValidationRerankModelInvalid:        cervii18n.FieldKnowledgeBaseRerankModelInvalid,

		knowledgebaseaction.ValidationQAQuestionRequired: cervii18n.FieldKnowledgeQAQuestionRequired,
		knowledgebaseaction.ValidationQAAnswerRequired:   cervii18n.FieldKnowledgeQAAnswerRequired,
		knowledgebaseaction.ValidationQAGroupInvalid:     cervii18n.FieldKnowledgeQAGroupInvalid,
		knowledgebaseaction.ValidationQAContentInvalid:   cervii18n.FieldKnowledgeQAContentInvalid,
		knowledgebaseaction.ValidationNameRequired:       cervii18n.FieldKnowledgeBaseNameRequired,
		knowledgebaseaction.ValidationNameTooLong:        cervii18n.FieldKnowledgeBaseNameTooLong,
		knowledgebaseaction.ValidationNameDuplicate:      cervii18n.FieldKnowledgeBaseNameDuplicate,
		knowledgebaseaction.ValidationCategoryInvalid:    cervii18n.FieldKnowledgeBaseCategoryInvalid,
		knowledgebaseaction.ValidationDescriptionTooLong: cervii18n.FieldKnowledgeBaseDescriptionTooLong,
		knowledgebaseaction.ValidationGroupNameRequired:  cervii18n.FieldKnowledgeGroupNameRequired,
		knowledgebaseaction.ValidationGroupNameTooLong:   cervii18n.FieldKnowledgeGroupNameTooLong,
		knowledgebaseaction.ValidationGroupNameDuplicate: cervii18n.FieldKnowledgeGroupNameDuplicate,
		knowledgebaseaction.ValidationGroupParentInvalid: cervii18n.FieldKnowledgeGroupParentInvalid,
	}
	return translateValidationFields(fields, keys)
}
