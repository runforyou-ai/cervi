package connector

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

// DifyKnowledgeFile 保存原始上传文档的文件名和内容。
type DifyKnowledgeFile struct {
	Name    string
	Content []byte
}

// DownloadFile 取得原文件签名地址并下载内容，缺少原文件时返回空值。
func (l *DifyKnowledgeDocumentLister) DownloadFile(ctx context.Context, config DifyKnowledgeBaseConfig, datasetID, documentID string) (*DifyKnowledgeFile, error) {
	document, err := l.Get(ctx, config, datasetID, documentID)
	if err != nil {
		return nil, err
	}
	if document.SourceType != "upload_file" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	path, err := difyKnowledgeDocumentPath(datasetID, documentID)
	if err != nil {
		return nil, err
	}
	request, err := newRequest(config.APIURL, path+"/download", config.APIKey, "Authorization", "Bearer ")
	if err != nil {
		return nil, err
	}
	var payload struct {
		URL string `json:"url"`
	}
	err = connectiontest.ReadHTTPResponse(ctx, l.client, request, func(body io.Reader) error {
		return json.NewDecoder(body).Decode(&payload)
	})
	if _, kind, _ := connectiontest.Details(err); kind == connectiontest.FailureNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// 签名地址使用独立请求，不携带 Dify API 密钥。
	downloadURL, err := url.Parse(payload.URL)
	if err == nil && payload.URL != "" && !downloadURL.IsAbs() {
		baseURL, parseErr := url.Parse(config.APIURL)
		if parseErr != nil {
			return nil, parseErr
		}
		downloadURL = baseURL.ResolveReference(downloadURL)
	}
	if err != nil || payload.URL == "" || downloadURL.Host == "" || (downloadURL.Scheme != "http" && downloadURL.Scheme != "https") {
		return nil, connectiontest.NewError(connectiontest.StageCapability, connectiontest.FailureProtocol, errors.New("invalid Dify document download URL"))
	}
	download, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL.String(), nil)
	if err != nil {
		return nil, err
	}
	// 文件地址可能重定向到对象存储，下载请求不含认证头。
	response, err := http.DefaultClient.Do(download)
	if err != nil {
		return nil, connectiontest.ClassifyTransportError(connectiontest.StageConnect, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, connectiontest.HTTPStatusError(response.StatusCode, "download knowledge document failed")
	}
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, connectiontest.ClassifyTransportError(connectiontest.StageConnect, err)
	}
	return &DifyKnowledgeFile{Name: document.FileName, Content: content}, nil
}
