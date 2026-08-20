//go:build !server

package apiproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/runforyou-ai/cervi/internal/appservice"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

var (
	_ appservice.Backend         = (*Backend)(nil)
	_ appservice.ServerConnector = (*Backend)(nil)
)

// Backend 将类型化应用服务调用转换为远程 HTTP 请求。
type Backend struct {
	connection *connection
}

// NewBackend 创建原生端使用的远程应用后端。
func NewBackend(store Store) (*Backend, error) {
	remoteConnection, err := newConnection(store)
	if err != nil {
		return nil, err
	}
	return &Backend{connection: remoteConnection}, nil
}

// InstallationStatus 返回远程服务端初始化状态和公开企业名称。
func (b *Backend) InstallationStatus(ctx context.Context, meta appservice.RequestMeta) (appservice.InstallationStatus, error) {
	var output appservice.InstallationStatus
	err := b.do(ctx, meta, http.MethodGet, "/installation/status", nil, nil, &output)
	return output, err
}

// Login 校验账号密码并返回登录令牌。
func (b *Backend) Login(ctx context.Context, meta appservice.RequestMeta, input appservice.LoginInput) (appservice.Auth, error) {
	var output appservice.Auth
	err := b.do(ctx, meta, http.MethodPost, "/auth/login", nil, input, &output)
	return output, err
}

// Logout 删除远程登录令牌。
func (b *Backend) Logout(ctx context.Context, meta appservice.RequestMeta) error {
	return b.do(ctx, meta, http.MethodPost, "/auth/logout", nil, nil, nil)
}

// LoadIdentity 返回当前远程登录身份。
func (b *Backend) LoadIdentity(ctx context.Context, meta appservice.RequestMeta) (appservice.Identity, error) {
	var output appservice.Identity
	err := b.do(ctx, meta, http.MethodGet, "/auth/identity", nil, nil, &output)
	return output, err
}

// LoadInbox 返回当前用户的远程收件箱。
func (b *Backend) LoadInbox(ctx context.Context, meta appservice.RequestMeta) (appservice.Inbox, error) {
	var output appservice.Inbox
	err := b.do(ctx, meta, http.MethodGet, "/inbox", nil, nil, &output)
	return output, err
}

// ListWebsiteChannels 返回远程网站渠道列表。
func (b *Backend) ListWebsiteChannels(ctx context.Context, meta appservice.RequestMeta, deleted bool) (appservice.WebsiteChannelList, error) {
	path := "/channels/website"
	if deleted {
		path += "/trash"
	}
	var output appservice.WebsiteChannelList
	err := b.do(ctx, meta, http.MethodGet, path, nil, nil, &output)
	return output, err
}

// GetWebsiteChannel 返回远程网站渠道详情。
func (b *Backend) GetWebsiteChannel(ctx context.Context, meta appservice.RequestMeta, channelID string) (appservice.WebsiteChannel, error) {
	var output appservice.WebsiteChannel
	err := b.do(ctx, meta, http.MethodGet, "/channels/website/"+url.PathEscape(channelID), nil, nil, &output)
	return output, err
}

// CreateWebsiteChannel 创建远程网站渠道。
func (b *Backend) CreateWebsiteChannel(ctx context.Context, meta appservice.RequestMeta, input appservice.WebsiteChannelInput) (appservice.WebsiteChannelSummary, error) {
	var output appservice.WebsiteChannelSummary
	err := b.do(ctx, meta, http.MethodPost, "/channels/website", nil, input, &output)
	return output, err
}

// UpdateWebsiteChannel 修改远程网站渠道。
func (b *Backend) UpdateWebsiteChannel(ctx context.Context, meta appservice.RequestMeta, channelID string, input appservice.WebsiteChannelInput) (appservice.WebsiteChannelSummary, error) {
	var output appservice.WebsiteChannelSummary
	err := b.do(ctx, meta, http.MethodPatch, "/channels/website/"+url.PathEscape(channelID), nil, input, &output)
	return output, err
}

// UpdateWebsiteChannelChatInterface 修改远程网站渠道聊天界面。
func (b *Backend) UpdateWebsiteChannelChatInterface(ctx context.Context, meta appservice.RequestMeta, channelID string, input appservice.WebsiteChannelChatInterfaceInput) (appservice.WebsiteChannelChatInterface, error) {
	var output appservice.WebsiteChannelChatInterface
	err := b.do(ctx, meta, http.MethodPatch, "/channels/website/"+url.PathEscape(channelID)+"/chat-interface", nil, input, &output)
	return output, err
}

// DeleteWebsiteChannel 将远程网站渠道移入回收站。
func (b *Backend) DeleteWebsiteChannel(ctx context.Context, meta appservice.RequestMeta, channelID string) error {
	return b.do(ctx, meta, http.MethodDelete, "/channels/website/"+url.PathEscape(channelID), nil, nil, nil)
}

// RestoreWebsiteChannel 恢复远程网站渠道。
func (b *Backend) RestoreWebsiteChannel(ctx context.Context, meta appservice.RequestMeta, channelID string) (appservice.WebsiteChannelSummary, error) {
	var output appservice.WebsiteChannelSummary
	err := b.do(ctx, meta, http.MethodPost, "/channels/website/"+url.PathEscape(channelID)+"/restore", nil, nil, &output)
	return output, err
}

// ListChannels 返回远程渠道选择项。
func (b *Backend) ListChannels(ctx context.Context, meta appservice.RequestMeta) (appservice.ChannelList, error) {
	var output appservice.ChannelList
	err := b.do(ctx, meta, http.MethodGet, "/channels", nil, nil, &output)
	return output, err
}

// ListUsers 返回远程企业成员列表。
func (b *Backend) ListUsers(ctx context.Context, meta appservice.RequestMeta, input appservice.UserListInput) (appservice.UserList, error) {
	query := url.Values{}
	setQuery(query, "query", input.Query)
	setOptionalQuery(query, "status", input.Status)
	setOptionalQuery(query, "role", input.Role)
	setPositiveQuery(query, "page", input.Page)
	setPositiveQuery(query, "pageSize", input.PageSize)
	var output appservice.UserList
	err := b.do(ctx, meta, http.MethodGet, "/users", query, nil, &output)
	return output, err
}

// GetUser 返回远程企业成员详情。
func (b *Backend) GetUser(ctx context.Context, meta appservice.RequestMeta, userID string) (appservice.DirectoryUser, error) {
	var output appservice.DirectoryUser
	err := b.do(ctx, meta, http.MethodGet, "/users/"+url.PathEscape(userID), nil, nil, &output)
	return output, err
}

// ListContacts 返回远程联系人列表。
func (b *Backend) ListContacts(ctx context.Context, meta appservice.RequestMeta, input appservice.ContactListInput) (appservice.ContactList, error) {
	path := "/contacts"
	if input.Deleted {
		path += "/trash"
	}
	query := url.Values{}
	setQuery(query, "query", input.Query)
	setOptionalQuery(query, "stage", input.Stage)
	setQuery(query, "channelId", input.ChannelID)
	setOptionalQuery(query, "methodType", input.MethodType)
	setQuery(query, "sort", string(input.Sort))
	setPositiveQuery(query, "page", input.Page)
	setPositiveQuery(query, "pageSize", input.PageSize)
	var output appservice.ContactList
	err := b.do(ctx, meta, http.MethodGet, path, query, nil, &output)
	return output, err
}

// GetContact 返回远程联系人详情。
func (b *Backend) GetContact(ctx context.Context, meta appservice.RequestMeta, contactID string) (appservice.Contact, error) {
	var output appservice.Contact
	err := b.do(ctx, meta, http.MethodGet, "/contacts/"+url.PathEscape(contactID), nil, nil, &output)
	return output, err
}

// CreateContact 创建远程联系人。
func (b *Backend) CreateContact(ctx context.Context, meta appservice.RequestMeta, input appservice.ContactInput) (appservice.Contact, error) {
	var output appservice.Contact
	err := b.do(ctx, meta, http.MethodPost, "/contacts", nil, input, &output)
	return output, err
}

// UpdateContact 修改远程联系人。
func (b *Backend) UpdateContact(ctx context.Context, meta appservice.RequestMeta, contactID string, input appservice.ContactInput) (appservice.Contact, error) {
	var output appservice.Contact
	err := b.do(ctx, meta, http.MethodPatch, "/contacts/"+url.PathEscape(contactID), nil, input, &output)
	return output, err
}

// DeleteContact 将远程联系人移入回收站。
func (b *Backend) DeleteContact(ctx context.Context, meta appservice.RequestMeta, contactID string) error {
	return b.do(ctx, meta, http.MethodDelete, "/contacts/"+url.PathEscape(contactID), nil, nil, nil)
}

// RestoreContact 恢复远程联系人。
func (b *Backend) RestoreContact(ctx context.Context, meta appservice.RequestMeta, contactID string) (appservice.Contact, error) {
	var output appservice.Contact
	err := b.do(ctx, meta, http.MethodPost, "/contacts/"+url.PathEscape(contactID)+"/restore", nil, nil, &output)
	return output, err
}

// GetS3Setting 返回远程对象存储设置。
func (b *Backend) GetS3Setting(ctx context.Context, meta appservice.RequestMeta) (appservice.S3Setting, error) {
	var output appservice.S3Setting
	err := b.do(ctx, meta, http.MethodGet, "/settings/storage/s3", nil, nil, &output)
	return output, err
}

// SaveS3Setting 保存远程对象存储设置。
func (b *Backend) SaveS3Setting(ctx context.Context, meta appservice.RequestMeta, input appservice.S3Setting) (appservice.S3Setting, error) {
	var output appservice.S3Setting
	err := b.do(ctx, meta, http.MethodPut, "/settings/storage/s3", nil, input, &output)
	return output, err
}

// TestS3Setting 测试远程对象存储连接。
func (b *Backend) TestS3Setting(ctx context.Context, meta appservice.RequestMeta, input appservice.S3Setting) error {
	return b.do(ctx, meta, http.MethodPost, "/settings/storage/s3/test", nil, input, nil)
}

// ServerURL 返回当前配置的企业服务器地址。
func (b *Backend) ServerURL(_ context.Context, _ appservice.RequestMeta) (string, error) {
	state := b.connection.currentState()
	if state == nil {
		return "", nil
	}
	return state.baseURL.String(), nil
}

// ProbeServer 检测企业服务器并返回公开企业名称，不保存地址。
func (b *Backend) ProbeServer(ctx context.Context, meta appservice.RequestMeta, serverURL string) (appservice.InstallationStatus, error) {
	state, status, err := b.inspectServer(ctx, meta, serverURL)
	if err != nil {
		return appservice.InstallationStatus{}, err
	}
	slog.Info("已检测到企业服务器", "server_url", state.baseURL.String(), "organization", status.OrganizationName)
	return status, nil
}

// ConnectServer 验证并保存企业服务器地址。
func (b *Backend) ConnectServer(ctx context.Context, meta appservice.RequestMeta, serverURL string) error {
	state, _, err := b.inspectServer(ctx, meta, serverURL)
	if err != nil {
		return err
	}
	if err := b.connection.store.SetServerURL(ctx, state.baseURL.String()); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.Warn("保存企业服务器配置失败", "server_url", state.baseURL.String(), "error", err)
		return localError(meta, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorServerConnectionSaveFailed, nil)
	}
	b.connection.mu.Lock()
	b.connection.state = state
	b.connection.mu.Unlock()
	slog.Info("企业服务器连接成功", "server_url", state.baseURL.String())
	return nil
}

// inspectServer 校验地址并读取远程初始化状态，不保存配置。
func (b *Backend) inspectServer(ctx context.Context, meta appservice.RequestMeta, serverURL string) (*remoteState, appservice.InstallationStatus, error) {
	parsed, err := parseServerURL(serverURL)
	if err != nil {
		var validationError *serverURLValidationError
		if !errors.As(err, &validationError) {
			return nil, appservice.InstallationStatus{}, fmt.Errorf("parse enterprise server URL: %w", err)
		}
		return nil, appservice.InstallationStatus{}, localError(meta, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorServerURLInvalid, map[string]cervii18n.Key{"serverUrl": validationError.messageKey})
	}
	state := newRemoteState(parsed)
	status, err := probeServer(ctx, state)
	if err != nil {
		if ctx.Err() != nil {
			return nil, appservice.InstallationStatus{}, ctx.Err()
		}
		slog.Warn("验证企业服务器失败", "server_url", parsed.String(), "error", err)
		return nil, appservice.InstallationStatus{}, localError(meta, http.StatusBadGateway, "SERVER_UNAVAILABLE", cervii18n.ErrorServerUnavailable, map[string]cervii18n.Key{"serverUrl": cervii18n.FieldServerURLNotCervi})
	}
	if !status.Installed || status.OrganizationName == "" {
		slog.Info("企业服务器尚未初始化", "server_url", parsed.String())
		return nil, appservice.InstallationStatus{}, localError(meta, http.StatusConflict, "SERVER_INITIALIZATION_REQUIRED", cervii18n.ErrorServerInitializationRequired, nil)
	}
	return state, status, nil
}

// do 向已连接的企业服务器发送 HTTP 请求。
func (b *Backend) do(ctx context.Context, meta appservice.RequestMeta, method, path string, query url.Values, input, output any) error {
	state := b.connection.currentState()
	if state == nil {
		return localError(meta, http.StatusPreconditionRequired, "SERVER_CONNECTION_REQUIRED", cervii18n.ErrorServerConnectionRequired, nil)
	}
	var body io.Reader
	if input != nil {
		payload, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode remote request: %w", err)
		}
		body = bytes.NewReader(payload)
	}
	rawQuery := ""
	if query != nil {
		rawQuery = query.Encode()
	}
	endpoint := remoteEndpoint(state.baseURL, path, rawQuery)
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return localError(meta, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorRemoteRequestCreateFailed, nil)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Language", string(meta.Locale))
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if meta.Token != "" {
		request.Header.Set("Authorization", "Bearer "+meta.Token)
	}
	response, err := state.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.Warn("企业服务器请求失败", "server_url", state.baseURL.String(), "method", method, "path", path, "error", err)
		return localError(meta, http.StatusBadGateway, "SERVER_UNAVAILABLE", cervii18n.ErrorServerConnectionFailed, nil)
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, maxResponseBytes)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var payload errorBody
		if err := json.NewDecoder(limited).Decode(&payload); err != nil {
			slog.Warn("解析企业服务器错误响应失败", "server_url", state.baseURL.String(), "method", method, "path", path, "status", response.StatusCode, "error", err)
			return &appservice.Error{Status: response.StatusCode, Code: "REMOTE_ERROR", Message: http.StatusText(response.StatusCode)}
		}
		return &appservice.Error{Status: response.StatusCode, Code: payload.Error.Code, Message: payload.Error.Message, Fields: payload.Error.Fields}
	}
	if output == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(limited).Decode(output); err != nil {
		slog.Warn("解析企业服务器响应失败", "server_url", state.baseURL.String(), "method", method, "path", path, "status", response.StatusCode, "error", err)
		return localError(meta, http.StatusBadGateway, "SERVER_INVALID_RESPONSE", cervii18n.ErrorServerConnectionFailed, nil)
	}
	return nil
}

// localError 把错误码转换为本地化业务错误。
func localError(meta appservice.RequestMeta, status int, code string, messageKey cervii18n.Key, fields map[string]cervii18n.Key) *appservice.Error {
	message, _ := cervii18n.Localize(string(meta.Locale), messageKey)
	return &appservice.Error{Status: status, Code: code, Message: message, Fields: cervii18n.LocalizeMap(string(meta.Locale), fields)}
}

// setQuery 在值非空时写入查询参数。
func setQuery(query url.Values, name, value string) {
	if value != "" {
		query.Set(name, value)
	}
}

// setOptionalQuery 在指针非空时写入查询参数。
func setOptionalQuery[T ~string](query url.Values, name string, value *T) {
	if value != nil {
		setQuery(query, name, string(*value))
	}
}

// setPositiveQuery 在值为正数时写入查询参数。
func setPositiveQuery(query url.Values, name string, value int) {
	if value > 0 {
		query.Set(name, strconv.Itoa(value))
	}
}
