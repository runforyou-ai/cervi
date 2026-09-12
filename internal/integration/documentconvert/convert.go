//go:build server

// Package documentconvert 通过 markitdown 服务把原件转换为 Markdown 正文。
package documentconvert

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

// Error 定义原件转换的语言无关失败原因码。
type Error struct {
	Code string `json:"code"`
}

// Error 返回语言无关的失败原因。
func (e *Error) Error() string { return "document convert: " + e.Code }

// Client 通过内部 HTTP 接口把原件转换为 Markdown 正文。
type Client struct {
	url  string
	http *http.Client
}

// NewClient 创建原件转换客户端。
func NewClient(url string) *Client {
	return &Client{url: strings.TrimRight(url, "/"), http: &http.Client{Timeout: 15 * time.Minute}}
}

// CheckConnection 在三秒内执行一次转换服务连接检查。
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

// Convert 流式提交原件并返回转换后的 Markdown 正文。
func (c *Client) Convert(ctx context.Context, name string, source io.Reader) (string, error) {
	if c.url == "" {
		return "", &Error{Code: "unavailable"}
	}
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	done := make(chan error, 1)
	go func() {
		part, err := multipartWriter.CreateFormFile("file", name)
		if err == nil {
			_, err = io.Copy(part, source)
		}
		if err == nil {
			err = multipartWriter.Close()
		}
		_ = writer.CloseWithError(err)
		done <- err
	}()
	var output struct {
		Markdown string `json:"markdown"`
	}
	err := c.call(ctx, multipartWriter.FormDataContentType(), reader, &output)
	_ = reader.CloseWithError(err)
	writeErr := <-done
	if err != nil {
		return "", err
	}
	return output.Markdown, writeErr
}

// call 发送转换请求，非 200 时只取出响应中的原因码。
func (c *Client) call(ctx context.Context, contentType string, body io.Reader, output any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/convert", body)
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
		return fmt.Errorf("decode convert response: %w", err)
	}
	return nil
}
