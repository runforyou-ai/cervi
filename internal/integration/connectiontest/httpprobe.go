package connectiontest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// maxResponseSize 限制外部 HTTP 响应的读取上限。
const maxResponseSize = 1 << 20

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
	response, err := client.Do(request.Clone(ctx))
	if err != nil {
		return ClassifyTransportError(StageConnect, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return HTTPStatusError(response.StatusCode)
	}
	if err := decode(io.LimitReader(response.Body, maxResponseSize)); err != nil {
		return NewError(StageCapability, FailureProtocol, err)
	}
	return nil
}

// AppendPath 在保留自定义基础路径的前提下追加接口路径。
func AppendPath(baseURL, path string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	parsed.RawPath = ""
	parsed.Path = strings.TrimSuffix(parsed.Path, "/") + "/" + strings.TrimLeft(path, "/")
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
