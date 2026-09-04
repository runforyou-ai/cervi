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

	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

const difyKnowledgeRetrievalTimeout = 30 * time.Second

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

// Retrieve 使用调用方提供的 Dify 检索配置执行查询。
func (r *DifyKnowledgeRetriever) Retrieve(
	ctx context.Context,
	config DifyKnowledgeBaseConfig,
	datasetID, query string,
	retrievalModel json.RawMessage,
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
	body, err := json.Marshal(struct {
		Query          string          `json:"query"`
		RetrievalModel json.RawMessage `json:"retrieval_model"`
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
		return nil, fmt.Errorf("retrieve dify knowledge base: %w", err)
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
			Score: difyKnowledgeRetrievalScore(item.Score),
		})
	}
	return records, nil
}

// difyKnowledgeRetrievalScore 把 Dify 关键词检索用于表示“未计算”的零分转换为空值。
func difyKnowledgeRetrievalScore(score *float64) *float64 {
	if score != nil && *score == 0 {
		return nil
	}
	return score
}
