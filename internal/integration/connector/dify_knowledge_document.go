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

const difyKnowledgeDocumentTimeout = 10 * time.Second

// DifyKnowledgeDocument 定义 Dify 知识文档读取结果。
type DifyKnowledgeDocument struct {
	ID        string
	Name      string
	Status    string
	WordCount *int
	HitCount  int
	CreatedAt *time.Time
}

// DifyKnowledgeDocumentPage 定义 Dify 知识库文档分页结果。
type DifyKnowledgeDocumentPage struct {
	Documents []DifyKnowledgeDocument
	Total     int
}

// DifyKnowledgeDocumentListInput 定义 Dify 知识文档列表查询条件。
type DifyKnowledgeDocumentListInput struct {
	Keyword  string
	Status   string
	Page     int
	PageSize int
}

// DifyKnowledgeDocumentSegment 定义 Dify 知识文档分段。
type DifyKnowledgeDocumentSegment struct {
	ID        string
	Position  int
	Content   string
	Answer    *string
	WordCount int
	HitCount  int
	Status    string
	CreatedAt *time.Time
}

// DifyKnowledgeDocumentSegmentPage 定义 Dify 知识文档分段分页结果。
type DifyKnowledgeDocumentSegmentPage struct {
	Segments []DifyKnowledgeDocumentSegment
	Total    int
}

// DifyKnowledgeDocumentSegmentListInput 定义 Dify 知识文档分段查询条件。
type DifyKnowledgeDocumentSegmentListInput struct {
	Keyword  string
	Page     int
	PageSize int
}

// DifyKnowledgeDocumentLister 读取 Dify 知识库中的文档。
type DifyKnowledgeDocumentLister struct {
	client connectiontest.HTTPDoer
}

// NewDifyKnowledgeDocumentLister 创建 Dify 知识文档读取器。
func NewDifyKnowledgeDocumentLister(client connectiontest.HTTPDoer) *DifyKnowledgeDocumentLister {
	return &DifyKnowledgeDocumentLister{client: client}
}

// List 读取指定 Dify 知识库的一页文档。
func (l *DifyKnowledgeDocumentLister) List(
	ctx context.Context,
	config DifyKnowledgeBaseConfig,
	datasetID string,
	input DifyKnowledgeDocumentListInput,
) (DifyKnowledgeDocumentPage, error) {
	ctx, cancel := context.WithTimeout(ctx, difyKnowledgeDocumentTimeout)
	defer cancel()

	datasetID = strings.TrimSpace(datasetID)
	if datasetID == "" {
		return DifyKnowledgeDocumentPage{}, connectiontest.NewError(
			connectiontest.StageCapability,
			connectiontest.FailureInvalidConfig,
			errors.New("dify knowledge base id is empty"),
		)
	}
	request, err := newRequest(
		config.APIURL,
		"datasets/"+url.PathEscape(datasetID)+"/documents",
		config.APIKey,
		"Authorization",
		"Bearer ",
	)
	if err != nil {
		return DifyKnowledgeDocumentPage{}, err
	}
	query := request.URL.Query()
	if input.Keyword != "" {
		query.Set("keyword", input.Keyword)
	}
	if input.Status != "" {
		query.Set("status", input.Status)
	}
	query.Set("page", fmt.Sprintf("%d", input.Page))
	query.Set("limit", fmt.Sprintf("%d", input.PageSize))
	request.URL.RawQuery = query.Encode()

	var payload struct {
		Data []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			DisplayStatus string `json:"display_status"`
			CreatedAt     *int64 `json:"created_at"`
		} `json:"data"`
		Page  *int `json:"page"`
		Limit *int `json:"limit"`
		Total *int `json:"total"`
	}
	err = connectiontest.ReadHTTPResponse(ctx, l.client, request, func(body io.Reader) error {
		if err := json.NewDecoder(body).Decode(&payload); err != nil {
			return fmt.Errorf("decode dify knowledge document response: %w", err)
		}
		return nil
	})
	if err != nil {
		return DifyKnowledgeDocumentPage{}, err
	}
	if payload.Data == nil || payload.Page == nil || payload.Limit == nil || payload.Total == nil ||
		*payload.Page <= 0 || *payload.Limit <= 0 || *payload.Total < 0 {
		return DifyKnowledgeDocumentPage{}, connectiontest.NewError(
			connectiontest.StageCapability,
			connectiontest.FailureProtocol,
			errors.New("dify knowledge document response does not contain valid pagination"),
		)
	}

	documents := make([]DifyKnowledgeDocument, 0, len(payload.Data))
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.ID)
		name := strings.TrimSpace(item.Name)
		status := strings.TrimSpace(item.DisplayStatus)
		if id == "" || name == "" || status == "" {
			return DifyKnowledgeDocumentPage{}, connectiontest.NewError(
				connectiontest.StageCapability,
				connectiontest.FailureProtocol,
				errors.New("dify knowledge document response contains invalid document fields"),
			)
		}
		documents = append(documents, DifyKnowledgeDocument{
			ID: id, Name: name, Status: status, CreatedAt: difyUnixTime(item.CreatedAt),
		})
	}

	return DifyKnowledgeDocumentPage{
		Documents: documents,
		Total:     *payload.Total,
	}, nil
}

// Get 读取指定 Dify 知识文档详情。
func (l *DifyKnowledgeDocumentLister) Get(
	ctx context.Context,
	config DifyKnowledgeBaseConfig,
	datasetID, documentID string,
) (DifyKnowledgeDocument, error) {
	ctx, cancel := context.WithTimeout(ctx, difyKnowledgeDocumentTimeout)
	defer cancel()

	path, err := difyKnowledgeDocumentPath(datasetID, documentID)
	if err != nil {
		return DifyKnowledgeDocument{}, err
	}
	request, err := newRequest(config.APIURL, path, config.APIKey, "Authorization", "Bearer ")
	if err != nil {
		return DifyKnowledgeDocument{}, err
	}
	query := request.URL.Query()
	query.Set("metadata", "without")
	request.URL.RawQuery = query.Encode()

	var payload struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		DisplayStatus string `json:"display_status"`
		WordCount     *int   `json:"word_count"`
		HitCount      int    `json:"hit_count"`
		CreatedAt     *int64 `json:"created_at"`
	}
	err = connectiontest.ReadHTTPResponse(ctx, l.client, request, func(body io.Reader) error {
		if err := json.NewDecoder(body).Decode(&payload); err != nil {
			return fmt.Errorf("decode dify knowledge document detail response: %w", err)
		}
		return nil
	})
	if err != nil {
		return DifyKnowledgeDocument{}, err
	}
	id := strings.TrimSpace(payload.ID)
	name := strings.TrimSpace(payload.Name)
	status := strings.TrimSpace(payload.DisplayStatus)
	if id == "" || name == "" || status == "" ||
		(payload.WordCount != nil && *payload.WordCount < 0) || payload.HitCount < 0 {
		return DifyKnowledgeDocument{}, connectiontest.NewError(
			connectiontest.StageCapability,
			connectiontest.FailureProtocol,
			errors.New("dify knowledge document detail response contains invalid fields"),
		)
	}
	return DifyKnowledgeDocument{
		ID: id, Name: name, Status: status, WordCount: payload.WordCount,
		HitCount: payload.HitCount, CreatedAt: difyUnixTime(payload.CreatedAt),
	}, nil
}

// ListSegments 读取指定 Dify 知识文档的一页分段。
func (l *DifyKnowledgeDocumentLister) ListSegments(
	ctx context.Context,
	config DifyKnowledgeBaseConfig,
	datasetID, documentID string,
	input DifyKnowledgeDocumentSegmentListInput,
) (DifyKnowledgeDocumentSegmentPage, error) {
	ctx, cancel := context.WithTimeout(ctx, difyKnowledgeDocumentTimeout)
	defer cancel()

	path, err := difyKnowledgeDocumentPath(datasetID, documentID)
	if err != nil {
		return DifyKnowledgeDocumentSegmentPage{}, err
	}
	request, err := newRequest(config.APIURL, path+"/segments", config.APIKey, "Authorization", "Bearer ")
	if err != nil {
		return DifyKnowledgeDocumentSegmentPage{}, err
	}
	query := request.URL.Query()
	if input.Keyword != "" {
		query.Set("keyword", input.Keyword)
	}
	query.Set("page", fmt.Sprintf("%d", input.Page))
	query.Set("limit", fmt.Sprintf("%d", input.PageSize))
	request.URL.RawQuery = query.Encode()

	var payload struct {
		Data []struct {
			ID        string  `json:"id"`
			Position  *int    `json:"position"`
			Content   string  `json:"content"`
			Answer    *string `json:"answer"`
			WordCount int     `json:"word_count"`
			HitCount  int     `json:"hit_count"`
			Status    string  `json:"status"`
			CreatedAt *int64  `json:"created_at"`
		} `json:"data"`
		Page  *int `json:"page"`
		Limit *int `json:"limit"`
		Total *int `json:"total"`
	}
	err = connectiontest.ReadHTTPResponse(ctx, l.client, request, func(body io.Reader) error {
		if err := json.NewDecoder(body).Decode(&payload); err != nil {
			return fmt.Errorf("decode dify knowledge document segment response: %w", err)
		}
		return nil
	})
	if err != nil {
		return DifyKnowledgeDocumentSegmentPage{}, err
	}
	if payload.Data == nil || payload.Page == nil || payload.Limit == nil || payload.Total == nil ||
		*payload.Page <= 0 || *payload.Limit <= 0 || *payload.Total < 0 {
		return DifyKnowledgeDocumentSegmentPage{}, connectiontest.NewError(
			connectiontest.StageCapability,
			connectiontest.FailureProtocol,
			errors.New("dify knowledge document segment response does not contain valid pagination"),
		)
	}

	segments := make([]DifyKnowledgeDocumentSegment, 0, len(payload.Data))
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.ID)
		status := strings.TrimSpace(item.Status)
		if id == "" || status == "" || item.Position == nil || *item.Position <= 0 ||
			item.WordCount < 0 || item.HitCount < 0 {
			return DifyKnowledgeDocumentSegmentPage{}, connectiontest.NewError(
				connectiontest.StageCapability,
				connectiontest.FailureProtocol,
				errors.New("dify knowledge document segment response contains invalid fields"),
			)
		}
		segments = append(segments, DifyKnowledgeDocumentSegment{
			ID: id, Position: *item.Position, Content: item.Content, Answer: item.Answer,
			WordCount: item.WordCount, HitCount: item.HitCount, Status: status,
			CreatedAt: difyUnixTime(item.CreatedAt),
		})
	}
	return DifyKnowledgeDocumentSegmentPage{Segments: segments, Total: *payload.Total}, nil
}

// difyKnowledgeDocumentPath 返回经过路径转义的 Dify 知识文档地址。
func difyKnowledgeDocumentPath(datasetID, documentID string) (string, error) {
	datasetID = strings.TrimSpace(datasetID)
	documentID = strings.TrimSpace(documentID)
	if datasetID == "" || documentID == "" {
		return "", connectiontest.NewError(
			connectiontest.StageCapability,
			connectiontest.FailureInvalidConfig,
			errors.New("dify knowledge base or document id is empty"),
		)
	}
	return "datasets/" + url.PathEscape(datasetID) + "/documents/" + url.PathEscape(documentID), nil
}

// difyUnixTime 把 Dify 秒级时间戳转换为 UTC 时间。
func difyUnixTime(value *int64) *time.Time {
	if value == nil || *value <= 0 {
		return nil
	}
	converted := time.Unix(*value, 0).UTC()
	return &converted
}
