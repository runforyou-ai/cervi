package connector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

const difyKnowledgeRetrievalTimeout = 30 * time.Second

const (
	difyKnowledgeIndexingTechniqueEconomy     = "economy"
	difyKnowledgeIndexingTechniqueHighQuality = "high_quality"
	difyKnowledgeSearchMethodKeyword          = "keyword_search"
	difyKnowledgeSearchMethodSemantic         = "semantic_search"
	difyKnowledgeSearchMethodFullText         = "full_text_search"
	difyKnowledgeSearchMethodHybrid           = "hybrid_search"
)

// DifyKnowledgeRetrievalRecord 定义 Dify 知识库检索命中项。
type DifyKnowledgeRetrievalRecord struct {
	DocumentID   string
	DocumentName string
	SegmentID    string
	Position     int
	Content      string
	Answer       *string
	Score        *float64
}

// DifyKnowledgeRetriever 检索 Dify 知识库。
type DifyKnowledgeRetriever struct {
	client connectiontest.HTTPDoer
}

// NewDifyKnowledgeRetriever 创建 Dify 知识库检索器。
func NewDifyKnowledgeRetriever(client connectiontest.HTTPDoer) *DifyKnowledgeRetriever {
	return &DifyKnowledgeRetriever{client: client}
}

// Retrieve 返回指定 Dify 知识库的检索命中项。
func (r *DifyKnowledgeRetriever) Retrieve(
	ctx context.Context,
	config DifyKnowledgeBaseConfig,
	datasetID, query string,
	options domain.KnowledgeRetrievalOptions,
) ([]DifyKnowledgeRetrievalRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, difyKnowledgeRetrievalTimeout)
	defer cancel()

	datasetID = strings.TrimSpace(datasetID)
	if datasetID == "" {
		return nil, connectiontest.NewError(
			connectiontest.StageCapability,
			connectiontest.FailureInvalidConfig,
			errors.New("dify knowledge base id is empty"),
		)
	}
	retrievalModel, searchMethod, err := r.loadRetrievalModel(ctx, config, datasetID, options)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(struct {
		Query          string          `json:"query"`
		RetrievalModel json.RawMessage `json:"retrieval_model,omitempty"`
	}{Query: query, RetrievalModel: retrievalModel})
	if err != nil {
		return nil, err
	}
	requestURL, err := connectiontest.AppendPath(
		config.APIURL,
		"datasets/"+url.PathEscape(datasetID)+"/retrieve",
	)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+config.APIKey)

	var payload struct {
		Records *[]struct {
			Segment struct {
				ID         string  `json:"id"`
				Position   *int    `json:"position"`
				DocumentID string  `json:"document_id"`
				Content    string  `json:"content"`
				Answer     *string `json:"answer"`
				Document   struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"document"`
			} `json:"segment"`
			Score *float64 `json:"score"`
		} `json:"records"`
	}
	err = connectiontest.ReadHTTPResponse(ctx, r.client, request, func(reader io.Reader) error {
		if err := json.NewDecoder(reader).Decode(&payload); err != nil {
			return fmt.Errorf("decode dify knowledge retrieval response: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("retrieve dify knowledge base with search method %q: %w", searchMethod, err)
	}
	if payload.Records == nil {
		return nil, connectiontest.NewError(
			connectiontest.StageCapability,
			connectiontest.FailureProtocol,
			errors.New("dify knowledge retrieval response does not contain records"),
		)
	}

	records := make([]DifyKnowledgeRetrievalRecord, 0, len(*payload.Records))
	for index, item := range *payload.Records {
		segmentID := strings.TrimSpace(item.Segment.ID)
		documentID := strings.TrimSpace(item.Segment.Document.ID)
		if documentID == "" {
			documentID = strings.TrimSpace(item.Segment.DocumentID)
		}
		documentName := strings.TrimSpace(item.Segment.Document.Name)
		invalidField := ""
		switch {
		case segmentID == "":
			invalidField = "segment.id"
		case documentID == "":
			invalidField = "segment.document_id"
		case item.Segment.Position == nil || *item.Segment.Position <= 0:
			invalidField = "segment.position"
		}
		if invalidField != "" {
			return nil, connectiontest.NewError(
				connectiontest.StageCapability,
				connectiontest.FailureProtocol,
				fmt.Errorf(
					"dify knowledge retrieval response record %d contains invalid %s",
					index+1,
					invalidField,
				),
			)
		}
		records = append(records, DifyKnowledgeRetrievalRecord{
			DocumentID: documentID, DocumentName: documentName,
			SegmentID: segmentID, Position: *item.Segment.Position,
			Content: item.Segment.Content, Answer: item.Segment.Answer,
			Score: difyKnowledgeRetrievalScore(searchMethod, item.Score),
		})
	}
	return records, nil
}

// difyKnowledgeRetrievalScore 把 Dify 关键词检索用于表示“未计算”的零分转换为空值。
func difyKnowledgeRetrievalScore(searchMethod string, score *float64) *float64 {
	if searchMethod == difyKnowledgeSearchMethodKeyword && score != nil && *score == 0 {
		return nil
	}
	return score
}

// loadRetrievalModel 读取 Dify 知识库配置并合并本次检索参数。
func (r *DifyKnowledgeRetriever) loadRetrievalModel(
	ctx context.Context,
	config DifyKnowledgeBaseConfig,
	datasetID string,
	options domain.KnowledgeRetrievalOptions,
) (json.RawMessage, string, error) {
	request, err := newRequest(
		config.APIURL,
		"datasets/"+url.PathEscape(datasetID),
		config.APIKey,
		"Authorization",
		"Bearer ",
	)
	if err != nil {
		return nil, "", err
	}
	var payload struct {
		IndexingTechnique string          `json:"indexing_technique"`
		RetrievalModel    json.RawMessage `json:"retrieval_model_dict"`
	}
	err = connectiontest.ReadHTTPResponse(ctx, r.client, request, func(reader io.Reader) error {
		if err := json.NewDecoder(reader).Decode(&payload); err != nil {
			return fmt.Errorf("decode dify knowledge base retrieval config: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, "", fmt.Errorf("read dify knowledge base retrieval config: %w", err)
	}

	indexingTechnique := strings.TrimSpace(payload.IndexingTechnique)
	switch indexingTechnique {
	case difyKnowledgeIndexingTechniqueEconomy, difyKnowledgeIndexingTechniqueHighQuality:
	default:
		return nil, "", connectiontest.NewError(
			connectiontest.StageCapability,
			connectiontest.FailureProtocol,
			fmt.Errorf("unsupported dify knowledge base indexing technique %q", indexingTechnique),
		)
	}

	searchMethod, valid := difyKnowledgeSearchMethod(options.Method)
	if !valid {
		return nil, "", connectiontest.NewError(
			connectiontest.StageCapability,
			connectiontest.FailureInvalidConfig,
			fmt.Errorf("unsupported knowledge retrieval method %q", options.Method),
		)
	}
	if indexingTechnique == difyKnowledgeIndexingTechniqueEconomy &&
		searchMethod != difyKnowledgeSearchMethodKeyword {
		return nil, searchMethod, connectiontest.NewError(
			connectiontest.StageCapability,
			connectiontest.FailureInvalidConfig,
			fmt.Errorf("dify economy knowledge base does not support search method %q", searchMethod),
		)
	}

	model := make(map[string]any)
	retrievalModel := bytes.TrimSpace(payload.RetrievalModel)
	if len(retrievalModel) > 0 && !bytes.Equal(retrievalModel, []byte("null")) {
		if err := json.Unmarshal(retrievalModel, &model); err != nil {
			return nil, searchMethod, connectiontest.NewError(
				connectiontest.StageCapability,
				connectiontest.FailureProtocol,
				fmt.Errorf("decode dify knowledge base retrieval model: %w", err),
			)
		}
	}
	model["search_method"] = searchMethod
	model["reranking_enable"] = options.RerankingEnabled
	model["top_k"] = options.TopK
	model["score_threshold_enabled"] = options.ScoreThresholdEnabled
	model["score_threshold"] = nil
	if options.ScoreThresholdEnabled {
		model["score_threshold"] = options.ScoreThreshold
	}
	encoded, err := json.Marshal(model)
	if err != nil {
		return nil, searchMethod, err
	}
	return encoded, searchMethod, nil
}

// difyKnowledgeSearchMethod 把统一检索方式转换为 Dify 检索方式。
func difyKnowledgeSearchMethod(method domain.KnowledgeRetrievalMethod) (string, bool) {
	switch method {
	case domain.KnowledgeRetrievalMethodKeyword:
		return difyKnowledgeSearchMethodKeyword, true
	case domain.KnowledgeRetrievalMethodSemantic:
		return difyKnowledgeSearchMethodSemantic, true
	case domain.KnowledgeRetrievalMethodFullText:
		return difyKnowledgeSearchMethodFullText, true
	case domain.KnowledgeRetrievalMethodHybrid:
		return difyKnowledgeSearchMethodHybrid, true
	default:
		return "", false
	}
}
