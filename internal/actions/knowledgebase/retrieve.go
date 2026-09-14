//go:build server

package knowledgebase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/common/searchtext"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/embedding"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
	"github.com/runforyou-ai/cervi/internal/integration/rerank"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// rrfConstant 是名次倒数融合的平滑常数。
const rrfConstant = 60

var (
	// ErrRetrievalNotReady 表示知识库还没有已发布的分段。
	ErrRetrievalNotReady = errors.New("knowledge base has no published segments")
	// ErrRetrievalQueryInvalid 表示检索内容为空或超出长度。
	ErrRetrievalQueryInvalid = errors.New("knowledge retrieval query is invalid")
)

type queryEmbedder interface {
	Embed(context.Context, embedding.Credential, string, int, []string) ([][]float32, error)
}
type candidateReranker interface {
	Rerank(context.Context, rerank.Credential, string, string, []string, int) ([]rerank.Score, error)
}

// RetrievalRecord 定义混合召回返回的一条分段、两路名次、融合分数和重排得分；未重排时 RerankScore 为空。
type RetrievalRecord struct {
	DocumentID     string
	DocumentName   string
	SegmentID      string
	SegmentBatchID string
	Position       int
	Content        string
	Score          float64
	RerankScore    *float64
	LexicalRank    int
	VectorRank     int
}

// RetrievalService 对知识库执行词法与向量并行召回、名次融合和重排，人工检索测试与 Agent 共用。
type RetrievalService struct {
	db       *bun.DB
	embedder queryEmbedder
	reranker candidateReranker
}

// NewRetrievalService 创建知识库检索服务。
func NewRetrievalService(db *bun.DB, embedder queryEmbedder, reranker candidateReranker) *RetrievalService {
	return &RetrievalService{db: db, embedder: embedder, reranker: reranker}
}

// knowledgeSource 固定一个知识库及其模型凭据，承担该库的召回与阅读。
type knowledgeSource struct {
	service  *RetrievalService
	base     servermodels.KnowledgeBase
	embed    embedding.Credential
	rerank   rerank.Credential
	rerankOn bool
}

// Retrieve 在当前企业的指定知识库中检索单条内容，用于知识库页面的检索测试。
func (s *RetrievalService) Retrieve(ctx context.Context, identity *servermodels.Identity, knowledgeBaseID, query string) ([]RetrievalRecord, error) {
	query = strings.TrimSpace(query)
	if query == "" || utf8.RuneCountInString(query) > domain.KnowledgeRetrievalQueryMaxLength {
		return nil, ErrRetrievalQueryInvalid
	}
	sources, err := s.sources(ctx, identity.Organization.ID, []string{knowledgeBaseID})
	if err != nil {
		return nil, err
	}
	published, err := s.db.NewSelect().Model((*servermodels.KnowledgeDocument)(nil)).
		Where("kd.knowledge_base_id = ? AND kd.segment_batch_id IS NOT NULL", knowledgeBaseID).Exists(ctx)
	if err != nil {
		return nil, err
	}
	if !published {
		return nil, ErrRetrievalNotReady
	}
	return sources[0].retrieve(ctx, query)
}

// Sources 按企业校验知识库并构造检索来源，供多知识库融合检索使用。
func (s *RetrievalService) Sources(ctx context.Context, organizationID string, knowledgeBaseIDs []string) ([]knowledgeretrieval.Source, error) {
	sources, err := s.sources(ctx, organizationID, knowledgeBaseIDs)
	if err != nil {
		return nil, err
	}
	output := make([]knowledgeretrieval.Source, 0, len(sources))
	for _, source := range sources {
		output = append(output, knowledgeretrieval.Source{
			ID: source.base.ID, Name: source.base.Name,
			Retrieve: func(ctx context.Context, query string) ([]knowledgeretrieval.Record, error) {
				records, err := source.retrieve(ctx, query)
				return retrievalRecords(records, true), err
			},
			Read: func(ctx context.Context, cursor knowledgeretrieval.Cursor, before, after int) ([]knowledgeretrieval.Record, error) {
				hits, err := readSegmentWindow(ctx, s.db, source.base.ID, cursor.DocumentID, cursor.SegmentID, before, after)
				if err != nil {
					return nil, err
				}
				records := make([]RetrievalRecord, 0, len(hits))
				for _, hit := range hits {
					records = append(records, RetrievalRecord{DocumentID: hit.DocumentID, DocumentName: hit.DocumentName, SegmentID: hit.ID, SegmentBatchID: hit.SegmentBatchID, Position: hit.Position, Content: hit.Content})
				}
				return retrievalRecords(records, false), nil
			},
		})
	}
	return output, nil
}

// sources 读取同企业知识库及其向量、重排模型凭据；任一知识库不存在时返回 ErrNotFound。
func (s *RetrievalService) sources(ctx context.Context, organizationID string, knowledgeBaseIDs []string) ([]*knowledgeSource, error) {
	for _, id := range knowledgeBaseIDs {
		if !common.ValidUUID(id) {
			return nil, ErrNotFound
		}
	}
	var bases []servermodels.KnowledgeBase
	if err := s.db.NewSelect().Model(&bases).Where("kb.organization_id = ? AND kb.id IN (?)", organizationID, bun.In(knowledgeBaseIDs)).Scan(ctx); err != nil {
		return nil, err
	}
	if len(bases) != len(knowledgeBaseIDs) {
		return nil, ErrNotFound
	}
	providerIDs := make([]string, 0, len(bases)*2)
	for _, base := range bases {
		providerIDs = append(providerIDs, base.EmbeddingProviderID)
		if base.RerankProviderID != "" {
			providerIDs = append(providerIDs, base.RerankProviderID)
		}
	}
	var providers []servermodels.AIProvider
	if err := s.db.NewSelect().Model(&providers).Where("aip.organization_id = ? AND aip.id IN (?)", organizationID, bun.In(providerIDs)).Scan(ctx); err != nil {
		return nil, err
	}
	byID := make(map[string]servermodels.AIProvider, len(providers))
	for _, provider := range providers {
		byID[provider.ID] = provider
	}
	sources := make([]*knowledgeSource, 0, len(bases))
	for _, id := range knowledgeBaseIDs {
		for _, base := range bases {
			if base.ID != id {
				continue
			}
			source := &knowledgeSource{service: s, base: base}
			provider, ok := byID[base.EmbeddingProviderID]
			if !ok {
				return nil, &embedding.Error{Code: "embedding_model_unavailable"}
			}
			baseURL, err := common.CompatibleModelBaseURL(provider.Brand, provider.APIURL)
			if err != nil {
				return nil, &embedding.Error{Code: "embedding_model_unavailable"}
			}
			source.embed = embedding.Credential{BaseURL: baseURL, APIKey: provider.APIKey}
			if base.RerankProviderID != "" {
				provider, ok := byID[base.RerankProviderID]
				if !ok {
					return nil, &rerank.Error{Code: "rerank_model_unavailable"}
				}
				source.rerank = rerank.Credential{Brand: provider.Brand, BaseURL: provider.APIURL, APIKey: provider.APIKey}
				source.rerankOn = true
			}
			sources = append(sources, source)
		}
	}
	return sources, nil
}

// retrieve 并行执行向量路和词法路，按名次倒数融合后交给重排模型，最后截取知识库的召回数量。
func (k *knowledgeSource) retrieve(ctx context.Context, query string) ([]RetrievalRecord, error) {
	started := time.Now()
	tsquery, lexical := searchtext.KnowledgeQuery(query)
	var vectorHits, lexicalHits []segmentHit
	var vectorErr, lexicalErr error
	var group sync.WaitGroup
	group.Go(func() {
		vectors, err := k.service.embedder.Embed(ctx, k.embed, k.base.EmbeddingModelIdentifier, k.base.EmbeddingDimension, []string{query})
		if err != nil {
			vectorErr = err
			return
		}
		vectorHits, vectorErr = searchSegmentsByVector(ctx, k.service.db, k.base.ID, k.base.EmbeddingDimension, vectors[0])
	})
	if lexical {
		group.Go(func() {
			lexicalHits, lexicalErr = searchSegmentsByText(ctx, k.service.db, k.base.ID, tsquery)
		})
	}
	group.Wait()
	if vectorErr != nil && (lexicalErr != nil || !lexical) {
		return nil, fmt.Errorf("retrieve knowledge base: %w", vectorErr)
	}
	if vectorErr != nil {
		slog.Warn("知识库向量召回失败", "knowledge_base_id", k.base.ID, "error", vectorErr)
	}
	if lexicalErr != nil {
		slog.Warn("知识库词法召回失败", "knowledge_base_id", k.base.ID, "error", lexicalErr)
	}
	type candidate struct {
		hit                     segmentHit
		score                   float64
		rerankScore             *float64
		lexicalRank, vectorRank int
	}
	fused := map[string]*candidate{}
	ordered := make([]*candidate, 0, len(lexicalHits)+len(vectorHits))
	for rank, hit := range lexicalHits {
		item := &candidate{hit: hit, lexicalRank: rank + 1}
		fused[hit.ID] = item
		ordered = append(ordered, item)
	}
	for rank, hit := range vectorHits {
		item := fused[hit.ID]
		if item == nil {
			item = &candidate{hit: hit}
			fused[hit.ID] = item
			ordered = append(ordered, item)
		}
		item.vectorRank = rank + 1
	}
	for _, item := range ordered {
		if item.lexicalRank > 0 {
			item.score += 1 / float64(rrfConstant+item.lexicalRank)
		}
		if item.vectorRank > 0 {
			item.score += 1 / float64(rrfConstant+item.vectorRank)
		}
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].score != ordered[j].score {
			return ordered[i].score > ordered[j].score
		}
		return ordered[i].hit.ID < ordered[j].hit.ID
	})
	// 重排只决定顺序：已打分的候选按相关性降序排在前面，供应商未返回得分的候选保持融合顺序接在后面。
	if k.rerankOn && len(ordered) > 0 {
		documents := make([]string, 0, len(ordered))
		for _, item := range ordered {
			documents = append(documents, item.hit.Content)
		}
		scores, err := k.service.reranker.Rerank(ctx, k.rerank, k.base.RerankModelIdentifier, query, documents, len(documents))
		if err != nil {
			return nil, fmt.Errorf("rerank knowledge candidates: %w", err)
		}
		reranked := make([]*candidate, 0, len(ordered))
		for _, score := range scores {
			if item := ordered[score.Index]; item.rerankScore == nil {
				relevance := score.Relevance
				item.rerankScore = &relevance
				reranked = append(reranked, item)
			}
		}
		sort.SliceStable(reranked, func(i, j int) bool { return *reranked[i].rerankScore > *reranked[j].rerankScore })
		for _, item := range ordered {
			if item.rerankScore == nil {
				reranked = append(reranked, item)
			}
		}
		ordered = reranked
	}
	ordered = ordered[:min(len(ordered), k.base.RetrievalCount)]
	records := make([]RetrievalRecord, 0, len(ordered))
	for _, item := range ordered {
		records = append(records, RetrievalRecord{
			DocumentID: item.hit.DocumentID, DocumentName: item.hit.DocumentName,
			SegmentID: item.hit.ID, SegmentBatchID: item.hit.SegmentBatchID, Position: item.hit.Position,
			Content: item.hit.Content, Score: item.score, RerankScore: item.rerankScore, LexicalRank: item.lexicalRank, VectorRank: item.vectorRank,
		})
	}
	slog.Info("知识库混合召回完成",
		"knowledge_base_id", k.base.ID, "lexical_count", len(lexicalHits), "vector_count", len(vectorHits),
		"reranked", k.rerankOn, "result_count", len(records), "duration_ms", time.Since(started).Milliseconds())
	return records, nil
}

// retrievalRecords 把召回或阅读结果映射为跨知识库融合使用的统一记录，只有召回结果携带分数，重排后以重排得分为准。
func retrievalRecords(records []RetrievalRecord, scored bool) []knowledgeretrieval.Record {
	output := make([]knowledgeretrieval.Record, 0, len(records))
	for _, record := range records {
		item := knowledgeretrieval.Record{
			DocumentID: record.DocumentID, DocumentName: record.DocumentName,
			SegmentID: record.SegmentID, Position: record.Position, Content: record.Content,
		}
		if scored {
			score := record.Score
			if record.RerankScore != nil {
				score = *record.RerankScore
			}
			item.Score = &score
		}
		output = append(output, item)
	}
	return output
}
