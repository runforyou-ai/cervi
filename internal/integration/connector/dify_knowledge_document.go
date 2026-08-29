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

// DifyKnowledgeDocument 定义 Dify 知识库文档列表项。
type DifyKnowledgeDocument struct {
	ID        string
	Name      string
	Status    string
	CreatedAt *time.Time
}

// DifyKnowledgeDocumentPage 定义 Dify 知识库文档分页结果。
type DifyKnowledgeDocumentPage struct {
	Documents []DifyKnowledgeDocument
	Total     int
}

// DifyKnowledgeDocumentLister 读取 Dify 知识库中的文档。
type DifyKnowledgeDocumentLister struct {
	client connectiontest.HTTPDoer
}

// NewDifyKnowledgeDocumentLister 创建 Dify 知识文档列表读取器。
func NewDifyKnowledgeDocumentLister(client connectiontest.HTTPDoer) *DifyKnowledgeDocumentLister {
	return &DifyKnowledgeDocumentLister{client: client}
}

// List 读取指定 Dify 知识库的一页文档。
func (l *DifyKnowledgeDocumentLister) List(
	ctx context.Context,
	config DifyKnowledgeBaseConfig,
	datasetID string,
	page, pageSize int,
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
	query.Set("page", fmt.Sprintf("%d", page))
	query.Set("limit", fmt.Sprintf("%d", pageSize))
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

// difyUnixTime 把 Dify 秒级时间戳转换为 UTC 时间。
func difyUnixTime(value *int64) *time.Time {
	if value == nil || *value <= 0 {
		return nil
	}
	converted := time.Unix(*value, 0).UTC()
	return &converted
}
