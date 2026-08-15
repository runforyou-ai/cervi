//go:build !server

// Package apiproxy 将本地 API 请求转发到企业 Cervi 服务端。
package apiproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

const (
	serverConnectionPath = "/server-connection"
	maxResponseBytes     = 1 << 20
)

type Store interface {
	// GetServerURL 读取已保存的企业服务器地址。
	GetServerURL(ctx context.Context) (string, error)
	// SetServerURL 保存企业服务器地址。
	SetServerURL(ctx context.Context, serverURL string) error
}

type Service struct {
	store Store
	mu    sync.RWMutex
	state *remoteState
}

type remoteState struct {
	baseURL *url.URL
	client  *http.Client
}

type serverRequest struct {
	ServerURL string `json:"serverUrl"`
}

type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type serverURLValidationError struct {
	messageKey cervii18n.Key
}

// Error 返回企业服务器地址的校验文案键。
func (e *serverURLValidationError) Error() string {
	return string(e.messageKey)
}

// NewService 创建 API 代理并加载企业服务器配置。
func NewService(store Store) (*Service, error) {
	service := &Service{store: store}
	serverURL, err := store.GetServerURL(context.Background())
	if err != nil {
		return nil, fmt.Errorf("read enterprise server configuration: %w", err)
	}
	if serverURL == "" {
		slog.Info("等待配置企业服务器")
		return service, nil
	}

	parsed, err := parseServerURL(serverURL)
	if err != nil {
		return nil, fmt.Errorf("parse saved enterprise server URL: %w", err)
	}
	state, err := newRemoteState(parsed)
	if err != nil {
		return nil, err
	}
	service.state = state
	slog.Info("已加载企业服务器配置", "server_url", parsed.String())
	return service, nil
}

// ServeHTTP 处理企业服务器配置请求或转发业务请求。
func (s *Service) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == serverConnectionPath {
		if request.Method == http.MethodPost {
			s.setConfiguration(writer, request)
			return
		}
		writer.Header().Set("Allow", "POST")
		writeError(writer, request, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", cervii18n.ErrorMethodNotAllowed, nil)
		return
	}

	s.proxy(writer, request)
}

// setConfiguration 验证并保存企业服务器地址。
func (s *Service) setConfiguration(writer http.ResponseWriter, request *http.Request) {
	var input serverRequest
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		writeError(writer, request, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorServerURLInvalid, nil)
		return
	}

	parsed, err := parseServerURL(input.ServerURL)
	if err != nil {
		validationError := err.(*serverURLValidationError)
		writeError(writer, request, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorServerURLInvalid, map[string]cervii18n.Key{
			"serverUrl": validationError.messageKey,
		})
		return
	}

	state, err := newRemoteState(parsed)
	if err != nil {
		slog.Warn("创建企业服务器连接失败", "server_url", parsed.String(), "error", err)
		writeError(writer, request, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorServerConnectionCreateFailed, nil)
		return
	}
	if err := probeServer(request.Context(), state); err != nil {
		slog.Warn("验证企业服务器失败", "server_url", parsed.String(), "error", err)
		writeError(writer, request, http.StatusBadGateway, "SERVER_UNAVAILABLE", cervii18n.ErrorServerUnavailable, map[string]cervii18n.Key{
			"serverUrl": cervii18n.FieldServerURLNotCervi,
		})
		return
	}
	if err := s.store.SetServerURL(request.Context(), parsed.String()); err != nil {
		slog.Warn("保存企业服务器配置失败", "server_url", parsed.String(), "error", err)
		writeError(writer, request, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorServerConnectionSaveFailed, nil)
		return
	}

	s.mu.Lock()
	s.state = state
	s.mu.Unlock()
	slog.Info("企业服务器连接成功", "server_url", parsed.String())
	writer.WriteHeader(http.StatusNoContent)
}

// proxy 将本地 API 请求转发到企业服务器。
func (s *Service) proxy(writer http.ResponseWriter, request *http.Request) {
	state := s.currentState()
	if state == nil {
		writeError(writer, request, http.StatusPreconditionRequired, "SERVER_CONNECTION_REQUIRED", cervii18n.ErrorServerConnectionRequired, nil)
		return
	}

	endpoint := remoteEndpoint(state.baseURL, request.URL.Path, request.URL.RawQuery)
	remoteRequest, err := http.NewRequestWithContext(request.Context(), request.Method, endpoint, request.Body)
	if err != nil {
		slog.Warn("创建企业服务器请求失败", "endpoint", endpoint, "error", err)
		writeError(writer, request, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorRemoteRequestCreateFailed, nil)
		return
	}
	copyRequestHeaders(remoteRequest.Header, request.Header)

	response, err := state.client.Do(remoteRequest)
	if err != nil {
		slog.Warn("企业服务器请求失败", "endpoint", endpoint, "error", err)
		writeError(writer, request, http.StatusBadGateway, "SERVER_UNAVAILABLE", cervii18n.ErrorServerConnectionFailed, nil)
		return
	}
	defer response.Body.Close()

	copyResponseHeaders(writer.Header(), response.Header)
	writer.WriteHeader(response.StatusCode)
	if _, err := io.Copy(writer, response.Body); err != nil {
		slog.Warn("转发企业服务器响应失败", "endpoint", endpoint, "error", err)
	}
}

// currentState 并发安全地读取当前远程连接状态。
func (s *Service) currentState() *remoteState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// newRemoteState 创建保留远程会话的 HTTP 客户端。
func newRemoteState(baseURL *url.URL) (*remoteState, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create remote cookie jar: %w", err)
	}
	return &remoteState{
		baseURL: baseURL,
		client: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
		},
	}, nil
}

// parseServerURL 解析并校验企业服务器基础地址。
func parseServerURL(value string) (*url.URL, error) {
	value = strings.TrimSpace(value)
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" {
		return nil, &serverURLValidationError{messageKey: cervii18n.FieldServerURLComplete}
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, &serverURLValidationError{messageKey: cervii18n.FieldServerURLBaseOnly}
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())) {
		return nil, &serverURLValidationError{messageKey: cervii18n.FieldServerURLHTTPSRequired}
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed, nil
}

// isLoopbackHost 判断主机是否指向本机回环地址。
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// probeServer 验证目标地址是否提供 Cervi API。
func probeServer(ctx context.Context, state *remoteState) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteEndpoint(state.baseURL, "/inbox", ""), nil)
	if err != nil {
		return err
	}
	response, err := state.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusUnauthorized && response.StatusCode != http.StatusConflict {
		return fmt.Errorf("服务器返回 HTTP %d", response.StatusCode)
	}

	var payload errorBody
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes)).Decode(&payload); err != nil {
		return errors.New("服务器响应格式不正确")
	}
	if payload.Error.Code != "AUTH_REQUIRED" && payload.Error.Code != "INSTALLATION_REQUIRED" {
		return errors.New("目标地址未提供 Cervi API")
	}
	return nil
}

// remoteEndpoint 拼接企业服务器 API 地址。
func remoteEndpoint(baseURL *url.URL, path, rawQuery string) string {
	endpoint := *baseURL
	endpoint.Path = strings.TrimRight(baseURL.Path, "/") + "/api" + path
	endpoint.RawQuery = rawQuery
	return endpoint.String()
}

// copyRequestHeaders 复制允许转发的请求头。
func copyRequestHeaders(target, source http.Header) {
	for _, name := range []string{"Accept", "Accept-Language", "Content-Type"} {
		if value := source.Get(name); value != "" {
			target.Set(name, value)
		}
	}
}

// copyResponseHeaders 复制允许返回的响应头。
func copyResponseHeaders(target, source http.Header) {
	for _, name := range []string{"Cache-Control", "Content-Language", "Content-Type", "ETag", "Vary"} {
		if values := source.Values(name); len(values) > 0 {
			target[name] = append([]string(nil), values...)
		}
	}
}

// writeError 返回统一格式的 API 代理错误。
func writeError(writer http.ResponseWriter, request *http.Request, status int, code string, messageKey cervii18n.Key, fields map[string]cervii18n.Key) {
	acceptLanguage := request.Header.Get("Accept-Language")
	message, language := cervii18n.Localize(acceptLanguage, messageKey)
	writer.Header().Set("Content-Language", language)
	writer.Header().Set("Vary", "Accept-Language")
	writeJSON(writer, status, errorBody{Error: apiError{
		Code:    code,
		Message: message,
		Fields:  cervii18n.LocalizeMap(acceptLanguage, fields),
	}})
}

// writeJSON 写入 JSON 响应及状态码。
func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		slog.Warn("写入 API 代理响应失败", "error", err)
	}
}
