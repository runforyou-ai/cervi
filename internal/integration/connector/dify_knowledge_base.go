package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

const (
	difyKnowledgeBasePageSize = 100
	difyKnowledgeBaseTimeout  = 10 * time.Second
)

// DifyKnowledgeBase 定义 Dify 知识库列表项。
type DifyKnowledgeBase struct {
	ID      string
	Name    string
	DocForm string
}

// DifyKnowledgeBaseConfig 定义 Dify 知识库列表访问配置。
type DifyKnowledgeBaseConfig struct {
	APIURL string
	APIKey string
}

// DifyKnowledgeBaseRetrievalModel 定义 Dify 知识库检索配置。
type DifyKnowledgeBaseRetrievalModel struct {
	SearchMethod          string   `json:"search_method"`
	TopK                  int      `json:"top_k"`
	ScoreThresholdEnabled bool     `json:"score_threshold_enabled"`
	ScoreThreshold        *float64 `json:"score_threshold"`
	RerankingEnable       bool     `json:"reranking_enable"`
	RerankingModel        struct {
		Provider string `json:"reranking_provider_name"`
		Name     string `json:"reranking_model_name"`
	} `json:"reranking_model"`
}

// DifyKnowledgeBaseDetail 定义 Dify 返回的知识库配置。
type DifyKnowledgeBaseDetail struct {
	IndexingTechnique      string
	DocumentCount          int
	WordCount              int
	EmbeddingModel         string
	EmbeddingModelProvider string
	RetrievalModel         DifyKnowledgeBaseRetrievalModel
	RetrievalModelJSON     json.RawMessage
}

// DifyKnowledgeBaseGetter 读取 Dify 知识库详情。
type DifyKnowledgeBaseGetter struct {
	client connectiontest.HTTPDoer
}

// NewDifyKnowledgeBaseGetter 创建 Dify 知识库详情读取器。
func NewDifyKnowledgeBaseGetter(client connectiontest.HTTPDoer) *DifyKnowledgeBaseGetter {
	return &DifyKnowledgeBaseGetter{client: client}
}

// Get 返回 Dify 知识库完整配置。
func (g *DifyKnowledgeBaseGetter) Get(ctx context.Context, config DifyKnowledgeBaseConfig, datasetID string) (DifyKnowledgeBaseDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, difyKnowledgeBaseTimeout)
	defer cancel()
	request, err := newRequest(config.APIURL, "datasets/"+url.PathEscape(datasetID), config.APIKey, "Authorization", "Bearer ")
	if err != nil {
		return DifyKnowledgeBaseDetail{}, err
	}
	var detail DifyKnowledgeBaseDetail
	err = connectiontest.ReadHTTPResponse(ctx, g.client, request, func(body io.Reader) error {
		var payload struct {
			IndexingTechnique      string          `json:"indexing_technique"`
			DocumentCount          int             `json:"document_count"`
			WordCount              int             `json:"word_count"`
			EmbeddingModel         string          `json:"embedding_model"`
			EmbeddingModelProvider string          `json:"embedding_model_provider"`
			RetrievalModel         json.RawMessage `json:"retrieval_model_dict"`
		}
		if err := json.NewDecoder(body).Decode(&payload); err != nil {
			return fmt.Errorf("decode dify knowledge base response: %w", err)
		}
		if len(payload.RetrievalModel) == 0 || string(payload.RetrievalModel) == "null" {
			return errors.New("dify knowledge base response does not contain retrieval_model_dict")
		}
		var retrievalModel DifyKnowledgeBaseRetrievalModel
		if err := json.Unmarshal(payload.RetrievalModel, &retrievalModel); err != nil {
			return fmt.Errorf("decode dify knowledge base retrieval model: %w", err)
		}
		detail = DifyKnowledgeBaseDetail{
			IndexingTechnique: payload.IndexingTechnique, DocumentCount: payload.DocumentCount,
			WordCount: payload.WordCount, EmbeddingModel: payload.EmbeddingModel,
			EmbeddingModelProvider: payload.EmbeddingModelProvider,
			RetrievalModel:         retrievalModel, RetrievalModelJSON: payload.RetrievalModel,
		}
		return nil
	})
	if err != nil {
		return DifyKnowledgeBaseDetail{}, err
	}
	return detail, nil
}

// DifyKnowledgeBaseLister 读取 Dify 连接可访问的知识库。
type DifyKnowledgeBaseLister struct {
	client connectiontest.HTTPDoer
}

// NewDifyKnowledgeBaseLister 创建 Dify 知识库列表读取器。
func NewDifyKnowledgeBaseLister(client connectiontest.HTTPDoer) *DifyKnowledgeBaseLister {
	return &DifyKnowledgeBaseLister{client: client}
}

// List 分页读取连接可访问的全部 Dify 知识库。
func (l *DifyKnowledgeBaseLister) List(ctx context.Context, config DifyKnowledgeBaseConfig) ([]DifyKnowledgeBase, error) {
	ctx, cancel := context.WithTimeout(ctx, difyKnowledgeBaseTimeout)
	defer cancel()

	knowledgeBases := make([]DifyKnowledgeBase, 0)
	for page := 1; ; page++ {
		items, hasMore, err := l.listPage(ctx, config, page)
		if err != nil {
			return nil, fmt.Errorf("list dify knowledge bases: %w", err)
		}
		knowledgeBases = append(knowledgeBases, items...)
		if !hasMore {
			return knowledgeBases, nil
		}
		if len(items) == 0 {
			return nil, connectiontest.NewError(
				connectiontest.StageCapability,
				connectiontest.FailureProtocol,
				errors.New("dify returned an empty page while has_more is true"),
			)
		}
	}
}

func (l *DifyKnowledgeBaseLister) listPage(
	ctx context.Context,
	config DifyKnowledgeBaseConfig,
	page int,
) ([]DifyKnowledgeBase, bool, error) {
	request, err := newRequest(config.APIURL, "datasets", config.APIKey, "Authorization", "Bearer ")
	if err != nil {
		return nil, false, err
	}
	query := request.URL.Query()
	query.Set("page", fmt.Sprintf("%d", page))
	query.Set("limit", fmt.Sprintf("%d", difyKnowledgeBasePageSize))
	request.URL.RawQuery = query.Encode()

	var payload struct {
		Data []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			DocForm string `json:"doc_form"`
		} `json:"data"`
		HasMore *bool `json:"has_more"`
	}
	err = connectiontest.ReadHTTPResponse(ctx, l.client, request, func(body io.Reader) error {
		if err := json.NewDecoder(body).Decode(&payload); err != nil {
			return fmt.Errorf("decode dify knowledge base response: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	if payload.Data == nil || payload.HasMore == nil {
		return nil, false, connectiontest.NewError(
			connectiontest.StageCapability,
			connectiontest.FailureProtocol,
			errors.New("dify knowledge base response does not contain data and has_more"),
		)
	}

	items := make([]DifyKnowledgeBase, 0, len(payload.Data))
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.ID)
		name := strings.TrimSpace(item.Name)
		docForm := strings.TrimSpace(item.DocForm)
		if id == "" || name == "" {
			return nil, false, connectiontest.NewError(
				connectiontest.StageCapability,
				connectiontest.FailureProtocol,
				errors.New("dify knowledge base response contains an empty id or name"),
			)
		}
		items = append(items, DifyKnowledgeBase{
			ID:      id,
			Name:    name,
			DocForm: docForm,
		})
	}

	return items, *payload.HasMore, nil
}
