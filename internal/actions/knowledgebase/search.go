//go:build server

package knowledgebase

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connector"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

type knowledgeRetriever interface {
	Retrieve(context.Context, connector.DifyKnowledgeBaseConfig, string, string, json.RawMessage) ([]connector.DifyKnowledgeRetrievalRecord, error)
}

type knowledgeBaseGetter interface {
	Get(context.Context, connector.DifyKnowledgeBaseConfig, string) (connector.DifyKnowledgeBaseDetail, error)
}

type knowledgeContextReader interface {
	Get(context.Context, connector.DifyKnowledgeBaseConfig, string, string) (connector.DifyKnowledgeDocument, error)
	ListSegmentsAround(context.Context, connector.DifyKnowledgeBaseConfig, string, string, string, int, int, int) ([]connector.DifyKnowledgeDocumentSegment, error)
}

type searchSourceRow struct {
	ID            string                                          `bun:"id"`
	Name          string                                          `bun:"name"`
	DatasetID     string                                          `bun:"dataset_id"`
	Configuration servermodels.IntegrationConnectionConfiguration `bun:"configuration,type:jsonb"`
}

// SearchService 构造知识库范围并执行统一检索。
type SearchService struct {
	db        *bun.DB
	getter    knowledgeBaseGetter
	retriever knowledgeRetriever
	reader    knowledgeContextReader
}

// NewSearchService 创建统一知识库检索服务。
func NewSearchService(db *bun.DB, getter knowledgeBaseGetter, retriever knowledgeRetriever, reader knowledgeContextReader) *SearchService {
	return &SearchService{db: db, getter: getter, retriever: retriever, reader: reader}
}

// ForOrganization 返回固定当前企业全部知识库范围的检索闭包。
func (s *SearchService) ForOrganization(ctx context.Context, organizationID string) (func(context.Context, knowledgeretrieval.Request) (knowledgeretrieval.Result, error), error) {
	rows := make([]searchSourceRow, 0)
	if err := s.db.NewSelect().
		TableExpr("knowledge_bases AS kb").
		ColumnExpr("kb.id::text AS id, kb.name, kb.external_resource_id AS dataset_id").
		ColumnExpr("ic.configuration AS configuration").
		Join("JOIN integration_connections AS ic ON ic.id = kb.integration_connection_id AND ic.organization_id = kb.organization_id").
		Where("kb.organization_id = ?", organizationID).
		Where("NULLIF(trim(kb.external_resource_id), '') IS NOT NULL").
		Where("ic.connector_type = ?", domain.IntegrationConnectionTypeDify).
		OrderExpr("kb.id ASC").
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("load knowledge search scope: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	sources := make([]knowledgeretrieval.Source, 0, len(rows))
	for _, row := range rows {
		source := row
		sources = append(sources, s.source(source.ID, source.Name, source.Configuration, source.DatasetID))
	}
	return func(searchCtx context.Context, request knowledgeretrieval.Request) (knowledgeretrieval.Result, error) {
		return knowledgeretrieval.Search(searchCtx, sources, request)
	}, nil
}

// ForKnowledgeBase 返回固定当前企业指定知识库范围的检索闭包。
func (s *SearchService) ForKnowledgeBase(ctx context.Context, organizationID, knowledgeBaseID string) (func(context.Context, knowledgeretrieval.Request) (knowledgeretrieval.Result, error), error) {
	knowledgeBase, err := loadKnowledgeBase(ctx, s.db, organizationID, knowledgeBaseID)
	if err != nil {
		return nil, err
	}
	access, err := loadDifyKnowledgeAccess(ctx, s.db, organizationID, knowledgeBaseID)
	if err != nil {
		return nil, err
	}
	source := s.source(knowledgeBase.ID, knowledgeBase.Name, servermodels.IntegrationConnectionConfiguration{
		APIURL: access.Config.APIURL, APIKey: access.Config.APIKey,
	}, access.DatasetID)
	return func(searchCtx context.Context, request knowledgeretrieval.Request) (knowledgeretrieval.Result, error) {
		return knowledgeretrieval.Search(searchCtx, []knowledgeretrieval.Source{source}, request)
	}, nil
}

// source 将 Dify 知识库统一转换为编排层数据源。
func (s *SearchService) source(id, name string, configuration servermodels.IntegrationConnectionConfiguration, datasetID string) knowledgeretrieval.Source {
	config := connector.DifyKnowledgeBaseConfig{APIURL: configuration.APIURL, APIKey: configuration.APIKey}
	var retrievalModel json.RawMessage
	var retrievalModelErr error
	var retrievalModelOnce sync.Once
	return knowledgeretrieval.Source{ID: id, Name: name, Retrieve: func(ctx context.Context, query string) ([]knowledgeretrieval.Record, error) {
		retrievalModelOnce.Do(func() {
			detail, err := s.getter.Get(ctx, config, datasetID)
			retrievalModelErr = err
			retrievalModel = detail.RetrievalModelJSON
		})
		if retrievalModelErr != nil {
			return nil, retrievalModelErr
		}
		records, err := s.retriever.Retrieve(ctx, config, datasetID, query, retrievalModel)
		if err != nil {
			return nil, err
		}
		result := make([]knowledgeretrieval.Record, 0, len(records))
		for _, record := range records {
			result = append(result, knowledgeretrieval.Record{
				DocumentID: record.DocumentID, DocumentName: record.DocumentName,
				SegmentID: record.SegmentID, Position: record.Position,
				Content: record.Content, Answer: record.Answer, Score: record.Score,
			})
		}
		return result, nil
	}, Read: func(ctx context.Context, cursor knowledgeretrieval.Cursor, before, after int) ([]knowledgeretrieval.Record, error) {
		document, err := s.reader.Get(ctx, config, datasetID, cursor.DocumentID)
		if err != nil {
			return nil, err
		}
		segments, err := s.reader.ListSegmentsAround(
			ctx, config, datasetID, cursor.DocumentID, cursor.SegmentID, cursor.Position, before, after,
		)
		if err != nil {
			return nil, err
		}
		result := make([]knowledgeretrieval.Record, 0, len(segments))
		for _, segment := range segments {
			result = append(result, knowledgeretrieval.Record{
				KnowledgeBaseID: id, KnowledgeBaseName: name,
				DocumentID: cursor.DocumentID, DocumentName: document.Name,
				SegmentID: segment.ID, Position: segment.Position,
				Content: segment.Content, Answer: segment.Answer,
				Matched: segment.ID == cursor.SegmentID && segment.Position == cursor.Position,
				Cursor: knowledgeretrieval.Cursor{
					KnowledgeBaseID: id, DocumentID: cursor.DocumentID,
					SegmentID: segment.ID, Position: segment.Position,
				},
			})
		}
		return result, nil
	}}
}
