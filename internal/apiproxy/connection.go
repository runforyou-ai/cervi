//go:build !server

// Package apiproxy 将类型化应用调用转换为企业 Cervi 服务端请求。
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
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/runforyou-ai/cervi/internal/appservice"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

const maxResponseBytes = 1 << 20

// Store 读写原生端连接的企业服务器地址。
type Store interface {
	// GetServerURL 读取已保存的企业服务器地址。
	GetServerURL(ctx context.Context) (string, error)
	// SetServerURL 保存企业服务器地址。
	SetServerURL(ctx context.Context, serverURL string) error
}

type connection struct {
	store Store
	mu    sync.RWMutex
	state *remoteState
}

type remoteState struct {
	baseURL *url.URL
	client  *http.Client
}

type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Kind    appservice.ErrorKind    `json:"kind,omitempty"`
	State   appservice.SessionState `json:"state,omitempty"`
	Message string                  `json:"message"`
	Fields  map[string]string       `json:"fields,omitempty"`
	Reason  string                  `json:"reason,omitempty"`
}

type serverURLValidationError struct {
	messageKey cervii18n.Key
}

// Error 返回企业服务器地址的校验文案键。
func (e *serverURLValidationError) Error() string {
	return string(e.messageKey)
}

// newConnection 读取已保存的企业服务器地址并建立远程连接。
func newConnection(store Store) (*connection, error) {
	result := &connection{store: store}
	serverURL, err := store.GetServerURL(context.Background())
	if err != nil {
		return nil, fmt.Errorf("read enterprise server configuration: %w", err)
	}
	if serverURL == "" {
		slog.Info("等待配置企业服务器")
		return result, nil
	}
	parsed, err := parseServerURL(serverURL)
	if err != nil {
		return nil, fmt.Errorf("parse saved enterprise server URL: %w", err)
	}
	result.state = newRemoteState(parsed)
	slog.Info("已加载企业服务器配置", "server_url", parsed.String())
	return result, nil
}

// currentState 返回当前远程连接状态。
func (c *connection) currentState() *remoteState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// newRemoteState 创建指向企业服务器的 HTTP 客户端。
func newRemoteState(baseURL *url.URL) *remoteState {
	return &remoteState{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// parseServerURL 校验企业服务器地址。
func parseServerURL(value string) (*url.URL, error) {
	value = strings.TrimSpace(value)
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" {
		return nil, &serverURLValidationError{messageKey: cervii18n.FieldServerURLComplete}
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, &serverURLValidationError{messageKey: cervii18n.FieldServerURLBaseOnly}
	}
	// 判断主机是否为回环地址。
	host := parsed.Hostname()
	loopback := strings.EqualFold(host, "localhost")
	if ip := net.ParseIP(host); ip != nil {
		loopback = loopback || ip.IsLoopback()
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && loopback) {
		return nil, &serverURLValidationError{messageKey: cervii18n.FieldServerURLHTTPSRequired}
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed, nil
}

// probeServer 读取企业服务器的初始化状态和公开企业名称。
func probeServer(ctx context.Context, state *remoteState) (appservice.InstallationStatus, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteEndpoint(state.baseURL, "/installation/status", ""), nil)
	if err != nil {
		return appservice.InstallationStatus{}, err
	}
	response, err := state.client.Do(request)
	if err != nil {
		return appservice.InstallationStatus{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return appservice.InstallationStatus{}, fmt.Errorf("server returned HTTP %d", response.StatusCode)
	}
	var payload struct {
		Installed        *bool  `json:"installed"`
		OrganizationName string `json:"organizationName"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes)).Decode(&payload); err != nil {
		return appservice.InstallationStatus{}, fmt.Errorf("decode installation status response: %w", err)
	}
	if payload.Installed == nil {
		return appservice.InstallationStatus{}, errors.New("target address does not serve the Cervi API")
	}
	return appservice.InstallationStatus{
		Installed:        *payload.Installed,
		OrganizationName: strings.TrimSpace(payload.OrganizationName),
	}, nil
}

// remoteEndpoint 拼接企业服务器 API 地址。
func remoteEndpoint(baseURL *url.URL, path, rawQuery string) string {
	endpoint := baseURL.Clone()
	endpoint.Path = strings.TrimRight(baseURL.Path, "/") + "/api" + path
	endpoint.RawQuery = rawQuery
	return endpoint.String()
}
