//go:build server

// Package rerank 调用重排模型接口为候选文本按与查询的相关性打分。
package rerank

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Credential 提供访问重排模型所需的供应商品牌、接口地址和密钥。
type Credential struct {
	Brand   string
	BaseURL string
	APIKey  string
}

// Score 表示一条候选文本的相关性得分，Index 为候选在请求中的下标。
type Score struct {
	Index     int
	Relevance float64
}

// Error 定义重排调用的语言无关失败原因码。
type Error struct {
	Code string
}

// Error 返回语言无关的失败原因。
func (e *Error) Error() string { return "rerank: " + e.Code }

// Client 通过供应商重排接口打分。
type Client struct{ http *http.Client }

// NewClient 创建重排客户端。
func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: time.Minute}}
}

// Rerank 按供应商接口格式提交查询与候选文本，返回候选下标与相关性得分；接口没有返回任何得分时视为失败。
func (c *Client) Rerank(ctx context.Context, credential Credential, model, query string, documents []string, topN int) ([]Score, error) {
	endpoint, err := Endpoint(credential.Brand, credential.BaseURL)
	if err != nil {
		return nil, &Error{Code: "rerank_model_unavailable"}
	}
	// 阿里云使用 DashScope 原生重排接口，其余品牌使用通用的 rerank 接口格式。
	var payload any
	if credential.Brand == "alibaba" {
		payload = map[string]any{
			"model":      model,
			"input":      map[string]any{"query": query, "documents": documents},
			"parameters": map[string]any{"return_documents": false, "top_n": topN},
		}
	} else {
		payload = map[string]any{"model": model, "query": query, "documents": documents, "top_n": topN}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, &Error{Code: "rerank_model_unavailable"}
	}
	request.Header.Set("Content-Type", "application/json")
	// 无凭据的自建或本机服务不携带鉴权头。
	if apiKey := strings.TrimSpace(credential.APIKey); apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}
	response, err := c.http.Do(request)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return nil, &Error{Code: "rerank_timeout"}
		}
		return nil, &Error{Code: "rerank_model_unavailable"}
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, &Error{Code: "rerank_model_unavailable"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &Error{Code: "rerank_failed"}
	}
	var decoded struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
		} `json:"results"`
		Output struct {
			Results []struct {
				Index          int     `json:"index"`
				RelevanceScore float64 `json:"relevance_score"`
			} `json:"results"`
		} `json:"output"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, &Error{Code: "rerank_failed"}
	}
	results := decoded.Results
	if credential.Brand == "alibaba" {
		results = decoded.Output.Results
	}
	if len(results) == 0 {
		return nil, &Error{Code: "rerank_failed"}
	}
	scores := make([]Score, 0, len(results))
	for _, item := range results {
		if item.Index < 0 || item.Index >= len(documents) {
			return nil, &Error{Code: "rerank_failed"}
		}
		scores = append(scores, Score{Index: item.Index, Relevance: item.RelevanceScore})
	}
	return scores, nil
}

// Endpoint 按品牌改写接口地址的路径后返回重排接口地址；阿里云使用 DashScope 原生重排路径。
func Endpoint(brand, value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("parse model base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("model base URL must include scheme and host")
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	if brand == "alibaba" {
		for _, suffix := range []string{"/compatible-mode/v1", "/api/v1", "/v1"} {
			path = strings.TrimSuffix(path, suffix)
		}
		path += "/api/v1/services/rerank/text-rerank/text-rerank"
	} else {
		path += "/rerank"
	}
	parsed.Path = path
	parsed.RawPath = ""
	return parsed.String(), nil
}
