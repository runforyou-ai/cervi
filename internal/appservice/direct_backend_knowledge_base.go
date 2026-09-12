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
	"github.com/runforyou-ai/cervi/internal/integration/documentconvert"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// knowledgeOps 持有知识库的 Action 和 Query。
type knowledgeOps struct {
	documentQuery        *knowledgebaseaction.DocumentQuery
	createDocuments      *knowledgebaseaction.CreateDocumentsAction
	documentProcessing   *knowledgebaseaction.DocumentProcessing
	documentConverter    *documentconvert.Client
	moveDocument         *knowledgebaseaction.MoveDocumentAction
	deleteDocument       *knowledgebaseaction.DeleteDocumentAction
	listQAEntries        *knowledgebaseaction.ListQAEntriesQuery
	getQAEntry           *knowledgebaseaction.GetQAEntryQuery
	saveQAEntry          *knowledgebaseaction.SaveQAEntryAction
	deleteQAEntry        *knowledgebaseaction.DeleteQAEntryAction
	listKnowledgeBases   *knowledgebaseaction.ListKnowledgeBasesQuery
	getKnowledgeBase     *knowledgebaseaction.GetKnowledgeBaseQuery
	createKnowledgeBase  *knowledgebaseaction.CreateKnowledgeBaseAction
	updateKnowledgeBase  *knowledgebaseaction.UpdateKnowledgeBaseAction
	deleteKnowledgeBase  *knowledgebaseaction.DeleteKnowledgeBaseAction
	createKnowledgeGroup *knowledgebaseaction.CreateKnowledgeGroupAction
	updateKnowledgeGroup *knowledgebaseaction.UpdateKnowledgeGroupAction
	deleteKnowledgeGroup *knowledgebaseaction.DeleteKnowledgeGroupAction
}

// newKnowledgeOps 创建知识库的业务实现依赖。
func newKnowledgeOps(db *bun.DB, taskEnqueuer servertask.TxEnqueuer, documentQuery *knowledgebaseaction.DocumentQuery, documentConverter *documentconvert.Client) knowledgeOps {
	return knowledgeOps{
		documentQuery:        documentQuery,
		createDocuments:      knowledgebaseaction.NewCreateDocumentsAction(db, taskEnqueuer),
		documentProcessing:   knowledgebaseaction.NewDocumentProcessing(db, taskEnqueuer),
		documentConverter:    documentConverter,
		moveDocument:         knowledgebaseaction.NewMoveDocumentAction(db),
		deleteDocument:       knowledgebaseaction.NewDeleteDocumentAction(db),
		listQAEntries:        knowledgebaseaction.NewListQAEntriesQuery(db),
		getQAEntry:           knowledgebaseaction.NewGetQAEntryQuery(db),
		saveQAEntry:          knowledgebaseaction.NewSaveQAEntryAction(db),
		deleteQAEntry:        knowledgebaseaction.NewDeleteQAEntryAction(db),
		listKnowledgeBases:   knowledgebaseaction.NewListKnowledgeBasesQuery(db),
		getKnowledgeBase:     knowledgebaseaction.NewGetKnowledgeBaseQuery(db),
		createKnowledgeBase:  knowledgebaseaction.NewCreateKnowledgeBaseAction(db),
		updateKnowledgeBase:  knowledgebaseaction.NewUpdateKnowledgeBaseAction(db),
		deleteKnowledgeBase:  knowledgebaseaction.NewDeleteKnowledgeBaseAction(db),
		createKnowledgeGroup: knowledgebaseaction.NewCreateKnowledgeGroupAction(db),
		updateKnowledgeGroup: knowledgebaseaction.NewUpdateKnowledgeGroupAction(db),
		deleteKnowledgeGroup: knowledgebaseaction.NewDeleteKnowledgeGroupAction(db),
	}
}

// ListKnowledgeBases 返回当前企业的知识库列表。
func (o *directOperations) ListKnowledgeBases(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (KnowledgeBaseList, error) {
	records, err := o.listKnowledgeBases.Execute(ctx, identity)
	if err != nil {
		return KnowledgeBaseList{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseListFailed, identity.Organization.ID, "")
	}
	knowledgeBases := make([]KnowledgeBase, 0, len(records))
	for _, record := range records {
		knowledgeBases = append(knowledgeBases, knowledgeBaseFromAction(record))
	}
	return KnowledgeBaseList{KnowledgeBases: knowledgeBases}, nil
}

// GetKnowledgeBase 返回当前企业中的知识库详情。
func (o *directOperations) GetKnowledgeBase(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, knowledgeBaseID string) (KnowledgeBase, error) {
	record, err := o.getKnowledgeBase.Execute(ctx, identity, knowledgeBaseID)
	if err != nil {
		return KnowledgeBase{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseReadFailed, identity.Organization.ID, knowledgeBaseID)
	}
	return knowledgeBaseFromAction(*record), nil
}

// CreateKnowledgeBase 创建企业知识库。
func (o *directOperations) CreateKnowledgeBase(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input KnowledgeBaseInput) (KnowledgeBase, error) {
	record, err := o.createKnowledgeBase.Execute(ctx, identity, knowledgebaseaction.Input{
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
		return KnowledgeBase{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseCreateFailed, identity.Organization.ID, "")
	}
	slog.Info("知识库创建成功", "organization_id", identity.Organization.ID, "knowledge_base_id", record.ID, "category", record.Category)
	return knowledgeBaseFromAction(*record), nil
}

// UpdateKnowledgeBase 修改企业知识库。
func (o *directOperations) UpdateKnowledgeBase(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, knowledgeBaseID string, input KnowledgeBaseInput) (KnowledgeBase, error) {
	record, err := o.updateKnowledgeBase.Execute(ctx, identity, knowledgeBaseID, knowledgebaseaction.Input{
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
		return KnowledgeBase{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseUpdateFailed, identity.Organization.ID, knowledgeBaseID)
	}
	slog.Info("知识库保存成功", "organization_id", identity.Organization.ID, "knowledge_base_id", record.ID, "category", record.Category)
	return knowledgeBaseFromAction(*record), nil
}

// DeleteKnowledgeBase 删除企业知识库。
func (o *directOperations) DeleteKnowledgeBase(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, knowledgeBaseID string) error {
	if err := o.deleteKnowledgeBase.Execute(ctx, identity, knowledgeBaseID); err != nil {
		return o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseDeleteFailed, identity.Organization.ID, knowledgeBaseID)
	}
	slog.Info("知识库删除成功", "organization_id", identity.Organization.ID, "knowledge_base_id", knowledgeBaseID)
	return nil
}

// CreateKnowledgeGroup 创建知识库分组。
func (o *directOperations) CreateKnowledgeGroup(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, knowledgeBaseID string, input KnowledgeGroupInput) (KnowledgeBase, error) {
	record, err := o.createKnowledgeGroup.Execute(ctx, identity, knowledgeBaseID, knowledgebaseaction.GroupInput{Name: input.Name, ParentID: input.ParentID})
	if err != nil {
		return KnowledgeBase{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeGroupCreateFailed, identity.Organization.ID, knowledgeBaseID)
	}
	slog.Info("知识库分组创建成功", "organization_id", identity.Organization.ID, "knowledge_base_id", knowledgeBaseID, "parent_group_id", input.ParentID)
	return knowledgeBaseFromAction(*record), nil
}

// UpdateKnowledgeGroup 修改知识库分组。
func (o *directOperations) UpdateKnowledgeGroup(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, knowledgeBaseID, groupID string, input KnowledgeGroupInput) (KnowledgeBase, error) {
	record, err := o.updateKnowledgeGroup.Execute(ctx, identity, knowledgeBaseID, groupID, knowledgebaseaction.GroupInput{Name: input.Name, ParentID: input.ParentID})
	if err != nil {
		return KnowledgeBase{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeGroupUpdateFailed, identity.Organization.ID, knowledgeBaseID)
	}
	slog.Info("知识库分组保存成功", "organization_id", identity.Organization.ID, "knowledge_base_id", knowledgeBaseID, "group_id", groupID)
	return knowledgeBaseFromAction(*record), nil
}

// DeleteKnowledgeGroup 删除不含子分组和问答的知识库分组。
func (o *directOperations) DeleteKnowledgeGroup(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, knowledgeBaseID, groupID string) (KnowledgeBase, error) {
	record, err := o.deleteKnowledgeGroup.Execute(ctx, identity, knowledgeBaseID, groupID)
	if err != nil {
		return KnowledgeBase{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeGroupDeleteFailed, identity.Organization.ID, knowledgeBaseID)
	}
	slog.Info("知识库分组删除成功", "organization_id", identity.Organization.ID, "knowledge_base_id", knowledgeBaseID, "group_id", groupID)
	return knowledgeBaseFromAction(*record), nil
}

// knowledgeBaseError 转换知识库领域错误。
func (o *directOperations) knowledgeBaseError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID, knowledgeBaseID string) error {
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

	if errors.Is(err, knowledgebaseaction.ErrSegmentsNotReady) {
		return ConflictError(meta, cervii18n.ErrorKnowledgeSegmentsNotReady, "segments_not_ready")
	}
	if errors.Is(err, knowledgebaseaction.ErrSegmentStale) {
		return ConflictError(meta, cervii18n.ErrorKnowledgeSegmentStale, "segment_stale")
	}
	if errors.Is(err, knowledgebaseaction.ErrSegmentQueryInvalid) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
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
