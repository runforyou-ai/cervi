package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
