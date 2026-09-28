//go:build server

package direct

import (
	"context"
	"errors"
	"log/slog"

	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	knowledgebaseaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/runforyou-ai/cervi/pkg/embedding"
	"github.com/runforyou-ai/cervi/pkg/rerank"
	"github.com/uptrace/bun"
)

// knowledgeOps 持有知识库的 Action 和 Query。
type knowledgeOps struct {
	documentQuery       *knowledgebaseaction.DocumentQuery
	createDocuments     *knowledgebaseaction.CreateDocumentsAction
	saveTextDocument    *knowledgebaseaction.SaveTextDocumentAction
	createWebDocument   *knowledgebaseaction.CreateWebDocumentAction
	renameDocument      *knowledgebaseaction.RenameDocumentAction
	documentProcessing  *knowledgebaseaction.DocumentProcessing
	deleteDocument      *knowledgebaseaction.DeleteDocumentAction
	listQAEntries       *knowledgebaseaction.ListQAEntriesQuery
	getQAEntry          *knowledgebaseaction.GetQAEntryQuery
	saveQAEntry         *knowledgebaseaction.SaveQAEntryAction
	qaProcessing        *knowledgebaseaction.QAProcessing
	deleteQAEntry       *knowledgebaseaction.DeleteQAEntryAction
	listKnowledgeBases  *knowledgebaseaction.ListKnowledgeBasesQuery
	getKnowledgeBase    *knowledgebaseaction.GetKnowledgeBaseQuery
	listBaseAgents      *knowledgebaseaction.ListKnowledgeBaseAgentsQuery
	createKnowledgeBase *knowledgebaseaction.CreateKnowledgeBaseAction
	updateKnowledgeBase *knowledgebaseaction.UpdateKnowledgeBaseAction
	deleteKnowledgeBase *knowledgebaseaction.DeleteKnowledgeBaseAction
	retrieval           *knowledgebaseaction.RetrievalService
}

// newKnowledgeOps 创建知识库的业务实现依赖。
func newKnowledgeOps(db *bun.DB, taskEnqueuer servertask.TxEnqueuer, documentQuery *knowledgebaseaction.DocumentQuery) knowledgeOps {
	return knowledgeOps{
		documentQuery:       documentQuery,
		createDocuments:     knowledgebaseaction.NewCreateDocumentsAction(db, taskEnqueuer),
		saveTextDocument:    knowledgebaseaction.NewSaveTextDocumentAction(db, taskEnqueuer),
		createWebDocument:   knowledgebaseaction.NewCreateWebDocumentAction(db, taskEnqueuer),
		renameDocument:      knowledgebaseaction.NewRenameDocumentAction(db),
		documentProcessing:  knowledgebaseaction.NewDocumentProcessing(db, taskEnqueuer),
		deleteDocument:      knowledgebaseaction.NewDeleteDocumentAction(db),
		listQAEntries:       knowledgebaseaction.NewListQAEntriesQuery(db),
		getQAEntry:          knowledgebaseaction.NewGetQAEntryQuery(db),
		saveQAEntry:         knowledgebaseaction.NewSaveQAEntryAction(db, taskEnqueuer),
		qaProcessing:        knowledgebaseaction.NewQAProcessing(db, taskEnqueuer),
		deleteQAEntry:       knowledgebaseaction.NewDeleteQAEntryAction(db),
		listKnowledgeBases:  knowledgebaseaction.NewListKnowledgeBasesQuery(db),
		getKnowledgeBase:    knowledgebaseaction.NewGetKnowledgeBaseQuery(db),
		listBaseAgents:      knowledgebaseaction.NewListKnowledgeBaseAgentsQuery(db),
		createKnowledgeBase: knowledgebaseaction.NewCreateKnowledgeBaseAction(db),
		updateKnowledgeBase: knowledgebaseaction.NewUpdateKnowledgeBaseAction(db, taskEnqueuer),
		deleteKnowledgeBase: knowledgebaseaction.NewDeleteKnowledgeBaseAction(db),
		retrieval:           knowledgebaseaction.NewRetrievalService(db, embedding.NewClient(), rerank.NewClient()),
	}
}

// RetrieveKnowledgeBase 在指定知识库中执行检索测试。
func (o *directOperations) RetrieveKnowledgeBase(ctx context.Context, meta appservice.RequestMeta, identity *servermodels.Identity, knowledgeBaseID string, input appservice.KnowledgeRetrievalInput) (appservice.KnowledgeRetrievalResult, error) {
	records, err := o.retrieval.Retrieve(ctx, identity, knowledgeBaseID, input.Query)
	if err != nil {
		return appservice.KnowledgeRetrievalResult{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeRetrievalFailed, identity.Organization.ID, knowledgeBaseID)
	}
	result := appservice.KnowledgeRetrievalResult{Records: make([]appservice.KnowledgeRetrievalRecord, 0, len(records))}
	for _, record := range records {
		result.Records = append(result.Records, appservice.KnowledgeRetrievalRecord{
			DocumentID: record.DocumentID, DocumentName: record.DocumentName,
			SegmentID: record.SegmentID, SegmentBatchID: record.SegmentBatchID, Position: record.Position,
			Context: record.Context, Content: record.Content, Answer: record.Answer, Score: record.Score,
		})
	}
	return result, nil
}

// ListKnowledgeBases 返回当前企业的知识库列表。
func (o *directOperations) ListKnowledgeBases(ctx context.Context, meta appservice.RequestMeta, identity *servermodels.Identity) (appservice.KnowledgeBaseList, error) {
	records, err := o.listKnowledgeBases.Execute(ctx, identity)
	if err != nil {
		return appservice.KnowledgeBaseList{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseListFailed, identity.Organization.ID, "")
	}
	knowledgeBases := make([]appservice.KnowledgeBase, 0, len(records))
	for _, record := range records {
		knowledgeBases = append(knowledgeBases, knowledgeBaseFromAction(record))
	}
	return appservice.KnowledgeBaseList{KnowledgeBases: knowledgeBases}, nil
}

// GetKnowledgeBase 返回当前企业中的知识库详情。
func (o *directOperations) GetKnowledgeBase(ctx context.Context, meta appservice.RequestMeta, identity *servermodels.Identity, knowledgeBaseID string) (appservice.KnowledgeBase, error) {
	record, err := o.getKnowledgeBase.Execute(ctx, identity, knowledgeBaseID)
	if err != nil {
		return appservice.KnowledgeBase{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseReadFailed, identity.Organization.ID, knowledgeBaseID)
	}
	return knowledgeBaseFromAction(*record), nil
}

// ListKnowledgeBaseAgents 返回当前配置版本绑定知识库的 AI 员工。
func (o *directOperations) ListKnowledgeBaseAgents(ctx context.Context, meta appservice.RequestMeta, identity *servermodels.Identity, knowledgeBaseID string) (appservice.KnowledgeBaseAgentList, error) {
	agents, err := o.listBaseAgents.Execute(ctx, identity, knowledgeBaseID)
	if err != nil {
		return appservice.KnowledgeBaseAgentList{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseReadFailed, identity.Organization.ID, knowledgeBaseID)
	}
	result := appservice.KnowledgeBaseAgentList{Agents: make([]appservice.KnowledgeBaseAgent, 0, len(agents))}
	for _, agent := range agents {
		result.Agents = append(result.Agents, appservice.KnowledgeBaseAgent{ID: agent.ID, DisplayName: agent.DisplayName, Status: appservice.UserStatus(agent.Status)})
	}
	return result, nil
}

// CreateKnowledgeBase 创建企业知识库。
func (o *directOperations) CreateKnowledgeBase(ctx context.Context, meta appservice.RequestMeta, identity *servermodels.Identity, input appservice.KnowledgeBaseInput) (appservice.KnowledgeBase, error) {
	record, err := o.createKnowledgeBase.Execute(ctx, identity, knowledgebaseaction.Input{
		Name: input.Name, Category: domain.KnowledgeBaseCategory(input.Category), Description: input.Description,
		EmbeddingProviderID:      input.EmbeddingProviderID,
		EmbeddingModelIdentifier: input.EmbeddingModelIdentifier,
		EmbeddingDimension:       input.EmbeddingDimension,
		ChunkLength:              input.ChunkLength,
		ChunkOverlap:             input.ChunkOverlap,
		RetrievalCount:           input.RetrievalCount,
		RetrievalScoreThreshold:  input.RetrievalScoreThreshold,
		RerankProviderID:         input.RerankProviderID,
		RerankModelIdentifier:    input.RerankModelIdentifier,
	})
	if err != nil {
		return appservice.KnowledgeBase{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseCreateFailed, identity.Organization.ID, "")
	}
	slog.Info("知识库创建成功", "organization_id", identity.Organization.ID, "knowledge_base_id", record.ID, "category", record.Category)
	return knowledgeBaseFromAction(*record), nil
}

// UpdateKnowledgeBase 修改企业知识库。
func (o *directOperations) UpdateKnowledgeBase(ctx context.Context, meta appservice.RequestMeta, identity *servermodels.Identity, knowledgeBaseID string, input appservice.KnowledgeBaseInput) (appservice.KnowledgeBase, error) {
	record, err := o.updateKnowledgeBase.Execute(ctx, identity, knowledgeBaseID, knowledgebaseaction.Input{
		Name: input.Name, Category: domain.KnowledgeBaseCategory(input.Category), Description: input.Description,
		EmbeddingProviderID:      input.EmbeddingProviderID,
		EmbeddingModelIdentifier: input.EmbeddingModelIdentifier,
		EmbeddingDimension:       input.EmbeddingDimension,
		ChunkLength:              input.ChunkLength,
		ChunkOverlap:             input.ChunkOverlap,
		RetrievalCount:           input.RetrievalCount,
		RetrievalScoreThreshold:  input.RetrievalScoreThreshold,
		RerankProviderID:         input.RerankProviderID,
		RerankModelIdentifier:    input.RerankModelIdentifier,
	})
	if err != nil {
		return appservice.KnowledgeBase{}, o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseUpdateFailed, identity.Organization.ID, knowledgeBaseID)
	}
	slog.Info("知识库保存成功", "organization_id", identity.Organization.ID, "knowledge_base_id", record.ID, "category", record.Category)
	return knowledgeBaseFromAction(*record), nil
}

// DeleteKnowledgeBase 删除企业知识库。
func (o *directOperations) DeleteKnowledgeBase(ctx context.Context, meta appservice.RequestMeta, identity *servermodels.Identity, knowledgeBaseID string) error {
	if err := o.deleteKnowledgeBase.Execute(ctx, identity, knowledgeBaseID); err != nil {
		return o.knowledgeBaseError(ctx, meta, err, cervii18n.ErrorKnowledgeBaseDeleteFailed, identity.Organization.ID, knowledgeBaseID)
	}
	slog.Info("知识库删除成功", "organization_id", identity.Organization.ID, "knowledge_base_id", knowledgeBaseID)
	return nil
}

// knowledgeBaseError 转换知识库领域错误。
func (o *directOperations) knowledgeBaseError(ctx context.Context, meta appservice.RequestMeta, err error, failureKey cervii18n.Key, organizationID, knowledgeBaseID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		return appservice.InvalidError(meta, cervii18n.ErrorValidationFailed, knowledgeBaseFieldKeys(validationError.Fields))
	}
	if errors.Is(err, identityaction.ErrInvalid) {
		return appservice.SessionError(meta, appservice.SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, fileaction.ErrFileNotFound) {
		return appservice.NotFoundError(meta, cervii18n.ErrorFileNotFound)
	}

	if errors.Is(err, knowledgebaseaction.ErrRetrievalQueryInvalid) {
		return appservice.InvalidError(meta, cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{"query": cervii18n.FieldKnowledgeRetrievalQueryInvalid})
	}
	if errors.Is(err, knowledgebaseaction.ErrRetrievalNotReady) {
		return appservice.ConflictError(meta, cervii18n.ErrorKnowledgeRetrievalNotReady, "retrieval_not_ready")
	}
	if embeddingError, ok := errors.AsType[*embedding.Error](err); ok {
		slog.Warn("知识库检索向量模型调用失败", "organization_id", organizationID, "knowledge_base_id", knowledgeBaseID, "code", embeddingError.Code)
		return appservice.UnavailableError(meta, cervii18n.ErrorKnowledgeEmbeddingUnavailable, nil)
	}
	if rerankError, ok := errors.AsType[*rerank.Error](err); ok {
		slog.Warn("知识库检索重排模型调用失败", "organization_id", organizationID, "knowledge_base_id", knowledgeBaseID, "code", rerankError.Code)
		return appservice.UnavailableError(meta, cervii18n.ErrorKnowledgeRerankUnavailable, nil)
	}
	if errors.Is(err, knowledgebaseaction.ErrSegmentsNotReady) {
		return appservice.ConflictError(meta, cervii18n.ErrorKnowledgeSegmentsNotReady, "segments_not_ready")
	}
	if errors.Is(err, knowledgebaseaction.ErrSegmentStale) {
		return appservice.ConflictError(meta, cervii18n.ErrorKnowledgeSegmentStale, "segment_stale")
	}
	if errors.Is(err, knowledgebaseaction.ErrSegmentQueryInvalid) || errors.Is(err, knowledgebaseaction.ErrPageSizeInvalid) {
		return appservice.InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
	}
	if errors.Is(err, knowledgebaseaction.ErrDocumentNotFound) {
		return appservice.NotFoundError(meta, cervii18n.ErrorKnowledgeDocumentNotFound)
	}
	if errors.Is(err, knowledgebaseaction.ErrDocumentUnsupported) {
		return appservice.InvalidError(meta, cervii18n.ErrorKnowledgeDocumentUnsupported, nil)
	}
	if errors.Is(err, knowledgebaseaction.ErrDocumentSourceUnsupported) {
		return appservice.InvalidError(meta, cervii18n.ErrorKnowledgeDocumentSourceUnsupported, nil)
	}
	if errors.Is(err, knowledgebaseaction.ErrDocumentURLDuplicate) {
		return appservice.ConflictError(meta, cervii18n.ErrorKnowledgeDocumentURLDuplicate, "document_url_duplicate")
	}
	if errors.Is(err, knowledgebaseaction.ErrDocumentBatchInvalid) {
		return appservice.InvalidError(meta, cervii18n.ErrorKnowledgeDocumentBatchInvalid, nil)
	}
	if errors.Is(err, knowledgebaseaction.ErrQANotFound) {
		return appservice.NotFoundError(meta, cervii18n.ErrorKnowledgeQANotFound)
	}
	if errors.Is(err, knowledgebaseaction.ErrQAUnsupported) {
		return appservice.InvalidError(meta, cervii18n.ErrorKnowledgeQAUnsupported, nil)
	}
	if errors.Is(err, knowledgebaseaction.ErrBaseHasContent) {
		return appservice.InvalidError(meta, cervii18n.ErrorKnowledgeBaseHasContent, nil)
	}
	if errors.Is(err, knowledgebaseaction.ErrNotFound) {
		return appservice.NotFoundError(meta, cervii18n.ErrorKnowledgeBaseNotFound)
	}
	attributes := []any{"organization_id", organizationID, "failure", failureKey, "error", err}
	if knowledgeBaseID != "" {
		attributes = append(attributes, "knowledge_base_id", knowledgeBaseID)
	}
	slog.Warn("知识库操作失败", attributes...)
	return appservice.FailedError(meta, failureKey)
}

// knowledgeBaseFromAction 转换知识库契约。
func knowledgeBaseFromAction(record knowledgebaseaction.Record) appservice.KnowledgeBase {
	return appservice.KnowledgeBase{
		ID: record.ID, Name: record.Name, Category: appservice.KnowledgeBaseCategory(record.Category), Description: record.Description,
		EmbeddingProviderID:      record.EmbeddingProviderID,
		EmbeddingModelIdentifier: record.EmbeddingModelIdentifier,
		EmbeddingDimension:       record.EmbeddingDimension,
		ChunkLength:              record.ChunkLength,
		ChunkOverlap:             record.ChunkOverlap,
		RetrievalCount:           record.RetrievalCount,
		RetrievalScoreThreshold:  record.RetrievalScoreThreshold,
		RerankProviderID:         record.RerankProviderID,
		RerankModelIdentifier:    record.RerankModelIdentifier,

		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
	}
}

// knowledgeBaseFieldKeys 把知识库校验错误码映射为本地化文案键。
func knowledgeBaseFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		knowledgebaseaction.ValidationEmbeddingModelInvalid:          cervii18n.FieldKnowledgeBaseEmbeddingModelInvalid,
		knowledgebaseaction.ValidationEmbeddingDimensionInvalid:      cervii18n.FieldKnowledgeBaseEmbeddingDimensionInvalid,
		knowledgebaseaction.ValidationChunkLengthInvalid:             cervii18n.FieldKnowledgeBaseChunkLengthInvalid,
		knowledgebaseaction.ValidationChunkOverlapInvalid:            cervii18n.FieldKnowledgeBaseChunkOverlapInvalid,
		knowledgebaseaction.ValidationRetrievalCountInvalid:          cervii18n.FieldKnowledgeBaseRetrievalCountInvalid,
		knowledgebaseaction.ValidationRetrievalScoreThresholdInvalid: cervii18n.FieldKnowledgeBaseRetrievalScoreThresholdInvalid,
		knowledgebaseaction.ValidationRerankModelInvalid:             cervii18n.FieldKnowledgeBaseRerankModelInvalid,

		knowledgebaseaction.ValidationDocumentTitleRequired:   cervii18n.FieldKnowledgeDocumentTitleRequired,
		knowledgebaseaction.ValidationDocumentTitleTooLong:    cervii18n.FieldKnowledgeDocumentTitleTooLong,
		knowledgebaseaction.ValidationDocumentContentRequired: cervii18n.FieldKnowledgeDocumentContentRequired,
		knowledgebaseaction.ValidationDocumentURLInvalid:      cervii18n.FieldKnowledgeDocumentURLInvalid,

		knowledgebaseaction.ValidationQAQuestionRequired: cervii18n.FieldKnowledgeQAQuestionRequired,
		knowledgebaseaction.ValidationQAAnswerRequired:   cervii18n.FieldKnowledgeQAAnswerRequired,
		knowledgebaseaction.ValidationQAContentInvalid:   cervii18n.FieldKnowledgeQAContentInvalid,
		knowledgebaseaction.ValidationNameRequired:       cervii18n.FieldKnowledgeBaseNameRequired,
		knowledgebaseaction.ValidationNameTooLong:        cervii18n.FieldKnowledgeBaseNameTooLong,
		knowledgebaseaction.ValidationNameDuplicate:      cervii18n.FieldKnowledgeBaseNameDuplicate,
		knowledgebaseaction.ValidationCategoryInvalid:    cervii18n.FieldKnowledgeBaseCategoryInvalid,
		knowledgebaseaction.ValidationDescriptionTooLong: cervii18n.FieldKnowledgeBaseDescriptionTooLong,
	}
	return translateValidationFields(fields, keys)
}
