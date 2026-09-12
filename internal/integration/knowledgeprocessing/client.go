package knowledgeprocessing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"strings"
	"time"
)

// Client 通过内部 HTTP 接口处理原件。
type Client struct {
	url  string
	http *http.Client
}

// NewClient 创建知识文档处理客户端。
func NewClient(url string) *Client {
	return &Client{url: strings.TrimRight(url, "/"), http: &http.Client{Timeout: 15 * time.Minute}}
}

// CheckConnection 在三秒内执行一次处理服务连接检查。
func (c *Client) CheckConnection(ctx context.Context) error {
	if c.url == "" {
		return &Error{Code: "unavailable"}
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+"/status", nil)
	if err != nil {
		return &Error{Code: "unavailable"}
	}
	response, err := c.http.Do(request)
	if err != nil {
		var networkError net.Error
		if errors.As(err, &networkError) && networkError.Timeout() {
			return &Error{Code: "connection_timeout"}
		}
		return &Error{Code: "unavailable"}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return &Error{Code: "unavailable"}
	}
	return nil
}

// Process 流式提交原件并读取分段处理结果。
func (c *Client) Process(ctx context.Context, input ProcessInput, credential EmbeddingCredential, name string, source io.Reader) (ProcessResult, error) {
	var output ProcessResult
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	done := make(chan error, 1)
	go func() {
		metadata, err := json.Marshal(struct {
			ProcessInput
			Embedding EmbeddingCredential `json:"embedding"`
		}{ProcessInput: input, Embedding: credential})
		if err == nil {
			err = multipartWriter.WriteField("metadata", string(metadata))
		}
		if err == nil {
			var part io.Writer
			part, err = multipartWriter.CreateFormFile("file", name)
			if err == nil {
				_, err = io.Copy(part, source)
			}
		}
		if err == nil {
			err = multipartWriter.Close()
		}
		_ = writer.CloseWithError(err)
		done <- err
	}()
	err := c.call(ctx, "/knowledge/process", multipartWriter.FormDataContentType(), reader, &output)
	_ = reader.CloseWithError(err)
	writeErr := <-done
	if err != nil {
		return output, err
	}
	return output, writeErr
}

// call 发送内部请求并仅接收约定的错误码。
func (c *Client) call(ctx context.Context, path, contentType string, body io.Reader, output any) error {
	if c.url == "" {
		return &Error{Code: "unavailable"}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+path, body)
	if err != nil {
		return &Error{Code: "unavailable"}
	}
	request.Header.Set("Content-Type", contentType)
	response, err := c.http.Do(request)
	if err != nil {
		var networkError net.Error
		if errors.As(err, &networkError) && networkError.Timeout() {
			return &Error{Code: "request_timeout"}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return &Error{Code: "unavailable"}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		var failure struct {
			Detail Error `json:"detail"`
		}
		if json.NewDecoder(response.Body).Decode(&failure) == nil && failure.Detail.Code != "" {
			return &failure.Detail
		}
		return &Error{Code: "service_failed"}
	}
	if err := json.NewDecoder(response.Body).Decode(output); err != nil {
		return fmt.Errorf("decode knowledge response: %w", err)
	}
	return nil
}
