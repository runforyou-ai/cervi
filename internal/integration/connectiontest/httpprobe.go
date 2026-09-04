package connectiontest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPDoer 定义外部 HTTP 请求需要的最小客户端契约。
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// NewHTTPClient 创建不跟随重定向的 HTTP 客户端。
func NewHTTPClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// ReadHTTPResponse 读取外部 HTTP 响应，并规范化传输、状态和响应契约错误。
func ReadHTTPResponse(ctx context.Context, client HTTPDoer, request *http.Request, decode func(io.Reader) error) error {
	body := ""
	if request.Body != nil {
		data, err := io.ReadAll(request.Body)
		if err != nil {
			return NewError(StageConnect, FailureProtocol, err)
		}
		body = string(data)
		request.Body = io.NopCloser(bytes.NewReader(data))
	}
	requestAttributes := []any{"method", request.Method, "url", request.URL.String()}
	if request.Method != http.MethodGet {
		requestAttributes = append(requestAttributes, "body", readableJSON([]byte(body)))
	}
	slog.Info("外部 HTTP 请求", requestAttributes...)
	startedAt := time.Now()
	response, err := client.Do(request.Clone(ctx))
	if err != nil {
		return ClassifyTransportError(StageConnect, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return NewError(StageCapability, FailureProtocol, err)
	}
	responseAttributes := []any{
		"method", request.Method,
		"url", request.URL.String(),
		"status_code", response.StatusCode,
		"duration_ms", time.Since(startedAt).Milliseconds(),
	}
	if request.Method != http.MethodGet {
		responseAttributes = append(responseAttributes, "body", readableJSON(responseBody))
	}
	slog.Info("外部 HTTP 响应", responseAttributes...)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return HTTPStatusError(response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	if err := decode(bytes.NewReader(responseBody)); err != nil {
		return NewError(StageCapability, FailureProtocol, err)
	}
	return nil
}

// readableJSON 把 JSON 中的 Unicode 转义转换为可直接阅读的字符。
func readableJSON(data []byte) string {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return string(data)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return string(data)
	}
	return string(encoded)
}

// AppendPath 在保留自定义基础路径的前提下追加接口路径。
func AppendPath(baseURL, path string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	escapedPath := strings.TrimLeft(path, "/")
	decodedPath, err := url.PathUnescape(escapedPath)
	if err != nil {
		return "", err
	}
	parsed.RawPath = strings.TrimSuffix(parsed.EscapedPath(), "/") + "/" + escapedPath
	parsed.Path = strings.TrimSuffix(parsed.Path, "/") + "/" + decodedPath
	return parsed.String(), nil
}

// InvalidConfigError 创建连接配置错误。
func InvalidConfigError(err error) error {
	return NewError(StageConnect, FailureInvalidConfig, err)
}

// ValidateDataList 校验带 data 数组的列表响应最小契约。
func ValidateDataList(reader io.Reader) error {
	var payload struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(reader).Decode(&payload); err != nil {
		return err
	}
	if len(payload.Data) == 0 || payload.Data[0] != '[' {
		return errors.New("list response does not contain a data array")
	}
	return nil
}
