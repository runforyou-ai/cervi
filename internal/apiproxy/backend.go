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
	"strings"
	"sync"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/clientsession"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

var (
	_ appservice.Backend         = (*Backend)(nil)
	_ appservice.ServerConnector = (*Backend)(nil)
)

// Backend 将类型化应用服务调用转换为远程 HTTP 请求。
type Backend struct {
	connection *connection
	sessions   *clientsession.Manager
	sessionMu  sync.Mutex
}

// NewBackend 创建原生端使用的远程应用后端。
func NewBackend(store Store, sessions *clientsession.Manager) (*Backend, error) {
	remoteConnection, err := newConnection(store)
	if err != nil {
		return nil, err
	}
	return &Backend{connection: remoteConnection, sessions: sessions}, nil
}

// InstallationStatus 不读取登录凭据并返回远程初始化状态。
func (b *Backend) InstallationStatus(ctx context.Context, meta appservice.RequestMeta) (appservice.InstallationStatus, error) {
	state := b.connection.currentState()
	if state == nil {
		return appservice.InstallationStatus{}, appservice.SessionError(meta, appservice.SessionStateConnect, cervii18n.ErrorServerConnectionRequired)
	}
	status, err := probeServer(ctx, state)
	if err == nil {
		return status, nil
	}
	if ctx.Err() != nil {
		return appservice.InstallationStatus{}, ctx.Err()
	}
	slog.Warn("检测已连接企业服务器失败", "server_url", state.baseURL.String(), "error", err)
	return appservice.InstallationStatus{}, appservice.UnavailableError(meta, cervii18n.ErrorServerConnectionFailed, nil)
}

// Login 校验账号密码并建立原生端登录会话。
func (b *Backend) Login(ctx context.Context, meta appservice.RequestMeta, input appservice.LoginInput) (appservice.Auth, error) {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	var output appservice.Auth
	if err := b.do(ctx, meta, http.MethodPost, "/auth/login", nil, input, &output); err != nil {
		return appservice.Auth{}, err
	}
	b.normalizeUser(&output.Identity.User)
	state := b.connection.currentState()
	if err := b.sessions.Establish(ctx, clientsession.Credential{
		ServerURL:      state.baseURL.String(),
		OrganizationID: output.Identity.Organization.ID,
		UserID:         output.Identity.User.ID,
		Token:          output.Token,
		ExpiresAt:      output.ExpiresAt,
	}); err != nil {
		slog.Warn("保存原生端登录凭据失败", "server_url", state.baseURL.String(), "user_id", output.Identity.User.ID, "error", err)
		return appservice.Auth{}, appservice.FailedError(meta, cervii18n.ErrorLoginFailed)
	}
	return appservice.Auth{Identity: output.Identity}, nil
}

// Logout 退出远程会话并清除原生端登录凭据。
func (b *Backend) Logout(ctx context.Context, meta appservice.RequestMeta) error {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	remoteErr := b.do(ctx, meta, http.MethodPost, "/auth/logout", nil, nil, nil)
	// 远程请求取消后仍清除本地凭据。
	if err := b.sessions.Clear(context.WithoutCancel(ctx)); err != nil {
		slog.Warn("清理原生端登录凭据失败", "error", err)
		return appservice.FailedError(meta, cervii18n.ErrorLogoutFailed)
	}
	return remoteErr
}

// ListContacts 返回远程联系人列表，回收站使用独立路径。
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

// normalizeOutput 按响应类型将远程响应中的相对文件地址转换为企业服务器绝对地址。
func (b *Backend) normalizeOutput(output any) {
	switch value := output.(type) {
	case *appservice.Identity:
		b.normalizeUser(&value.User)
	case *appservice.CurrentUser:
		b.normalizeUser(value)
	case *appservice.File:
		b.normalizeFile(value)
	case *appservice.FileUpload:
		b.normalizeFile(&value.File)
		value.Request.URL = b.absoluteContentURL(value.Request.URL)
	case *appservice.MemberOptionList:
		for index := range value.Members {
			value.Members[index].AvatarURL = b.absoluteContentURL(value.Members[index].AvatarURL)
		}
	case *appservice.TeamMemberCandidateList:
		for index := range value.Members {
			value.Members[index].AvatarURL = b.absoluteContentURL(value.Members[index].AvatarURL)
		}
	case *appservice.Inbox:
		for index := range value.Conversations {
			value.Conversations[index].ContactAvatarURL = b.absoluteContentURL(value.Conversations[index].ContactAvatarURL)
		}
	}
}

// normalizeUser 将服务端相对头像地址转换为企业服务器绝对地址。
func (b *Backend) normalizeUser(user *appservice.CurrentUser) {
	user.AvatarURL = b.absoluteContentURL(user.AvatarURL)
}

// normalizeFile 将服务端相对文件地址转换为企业服务器绝对地址。
func (b *Backend) normalizeFile(file *appservice.File) {
	file.ContentURL = b.absoluteContentURL(file.ContentURL)
}

// absoluteContentURL 为原生端补全企业服务器文件地址。
func (b *Backend) absoluteContentURL(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err == nil && parsed.IsAbs() {
		return parsed.String()
	}
	state := b.connection.currentState()
	if state == nil {
		return value
	}
	endpoint := *state.baseURL
	endpoint.Path = strings.TrimRight(state.baseURL.Path, "/") + "/" + strings.TrimLeft(value, "/")
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	return endpoint.String()
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
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	state, _, err := b.inspectServer(ctx, meta, serverURL)
	if err != nil {
		return err
	}
	current := b.connection.currentState()
	changed := current == nil || current.baseURL.String() != state.baseURL.String()
	if changed {
		if err := b.sessions.Clear(ctx); err != nil {
			slog.Warn("切换企业服务器前清理登录凭据失败", "server_url", state.baseURL.String(), "error", err)
			return appservice.FailedError(meta, cervii18n.ErrorServerConnectionSaveFailed)
		}
	}
	if err := b.connection.store.SetServerURL(ctx, state.baseURL.String()); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.Warn("保存企业服务器配置失败", "server_url", state.baseURL.String(), "error", err)
		return appservice.FailedError(meta, cervii18n.ErrorServerConnectionSaveFailed)
	}
	b.connection.mu.Lock()
	b.connection.state = state
	b.connection.mu.Unlock()
	slog.Info("企业服务器连接成功", "server_url", state.baseURL.String(), "changed", changed)
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
		return nil, appservice.InstallationStatus{}, appservice.InvalidError(meta, cervii18n.ErrorServerURLInvalid, map[string]cervii18n.Key{"serverUrl": validationError.messageKey})
	}
	state := newRemoteState(parsed)
	status, err := probeServer(ctx, state)
	if err != nil {
		if ctx.Err() != nil {
			return nil, appservice.InstallationStatus{}, ctx.Err()
		}
		slog.Warn("验证企业服务器失败", "server_url", parsed.String(), "error", err)
		return nil, appservice.InstallationStatus{}, appservice.UnavailableError(meta, cervii18n.ErrorServerUnavailable, map[string]cervii18n.Key{"serverUrl": cervii18n.FieldServerURLNotCervi})
	}
	if !status.Installed || status.OrganizationName == "" {
		slog.Info("企业服务器尚未初始化", "server_url", parsed.String())
		return nil, appservice.InstallationStatus{}, appservice.InvalidError(meta, cervii18n.ErrorServerInitializationRequired, nil)
	}
	return state, status, nil
}

// do 向已连接的企业服务器发送 HTTP 请求。
func (b *Backend) do(ctx context.Context, meta appservice.RequestMeta, method, path string, query url.Values, input, output any) error {
	state := b.connection.currentState()
	if state == nil {
		return appservice.SessionError(meta, appservice.SessionStateConnect, cervii18n.ErrorServerConnectionRequired)
	}
	credential, authenticated := b.sessions.Current(ctx, state.baseURL.String())
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
		return appservice.FailedError(meta, cervii18n.ErrorRemoteRequestCreateFailed)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Language", string(meta.Locale))
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if authenticated {
		request.Header.Set("Authorization", "Bearer "+credential.Token)
	}
	response, err := state.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.Warn("企业服务器请求失败", "server_url", state.baseURL.String(), "method", method, "path", path, "error", err)
		return appservice.UnavailableError(meta, cervii18n.ErrorServerConnectionFailed, nil)
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, maxResponseBytes)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var payload errorBody
		if err := json.NewDecoder(limited).Decode(&payload); err != nil {
			slog.Warn("解析企业服务器错误响应失败", "server_url", state.baseURL.String(), "method", method, "path", path, "status", response.StatusCode, "error", err)
			return &appservice.Error{Kind: appservice.ErrorKindFailed, Message: http.StatusText(response.StatusCode)}
		}
		sessionState := payload.Error.State
		if sessionState == appservice.SessionStateSetup {
			slog.Info("远端要求初始化，改为连接企业服务器")
			sessionState = appservice.SessionStateConnect
		}
		if sessionState == appservice.SessionStateLogin && authenticated {
			if err := b.sessions.ClearIfCurrent(ctx, credential); err != nil {
				slog.Warn("登录凭据失效后清理本地会话失败", "server_url", state.baseURL.String(), "error", err)
			}
		}
		return &appservice.Error{Kind: payload.Error.Kind, State: sessionState, Message: payload.Error.Message, Fields: payload.Error.Fields, Reason: payload.Error.Reason}
	}
	if output == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(limited).Decode(output); err != nil {
		slog.Warn("解析企业服务器响应失败", "server_url", state.baseURL.String(), "method", method, "path", path, "status", response.StatusCode, "error", err)
		return appservice.UnavailableError(meta, cervii18n.ErrorServerConnectionFailed, nil)
	}
	return nil
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
