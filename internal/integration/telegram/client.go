// Package telegram 提供 Telegram Bot API 的最小安全适配器。
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

const (
	defaultBaseURL  = "https://api.telegram.org"
	maxResponseSize = 1 << 20
)

var botTokenPattern = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]+$`)

// Bot 描述 getMe 返回的机器人身份。
type Bot struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

// Webhook 定义 setWebhook 所需字段。
type Webhook struct {
	URL    string
	Secret string
}

// BotAPI 定义渠道操作依赖的 Telegram API 能力。
type BotAPI interface {
	GetMe(context.Context, string) (Bot, error)
	SetWebhook(context.Context, string, Webhook) error
	DeleteWebhook(context.Context, string) error
}

// Option 配置 Telegram 客户端。
type Option func(*Client)

// WithBaseURL 覆盖 Bot API 基础地址，供受控环境和测试使用。
func WithBaseURL(baseURL string) Option {
	return func(client *Client) {
		client.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// Client 调用 Telegram Bot API，且不会在错误中暴露包含 Token 的 URL。
type Client struct {
	httpClient connectiontest.HTTPDoer
	baseURL    string
}

// NewClient 创建 Telegram Bot API 客户端。
func NewClient(httpClient connectiontest.HTTPDoer, options ...Option) *Client {
	if httpClient == nil {
		httpClient = connectiontest.NewHTTPClient()
	}
	client := &Client{httpClient: httpClient, baseURL: defaultBaseURL}
	for _, option := range options {
		option(client)
	}
	return client
}

// GetMe 校验 Token 并返回机器人身份。
func (c *Client) GetMe(ctx context.Context, token string) (Bot, error) {
	bot := Bot{}
	if err := c.call(ctx, token, "getMe", struct{}{}, &bot); err != nil {
		return Bot{}, err
	}
	if bot.ID <= 0 || !bot.IsBot || strings.TrimSpace(bot.FirstName) == "" {
		return Bot{}, protocolError()
	}
	return bot, nil
}

// SetWebhook 注册当前渠道的 Webhook。
func (c *Client) SetWebhook(ctx context.Context, token string, webhook Webhook) error {
	var accepted bool
	err := c.call(ctx, token, "setWebhook", struct {
		URL                string   `json:"url"`
		SecretToken        string   `json:"secret_token"`
		AllowedUpdates     []string `json:"allowed_updates"`
		DropPendingUpdates bool     `json:"drop_pending_updates"`
	}{
		URL: webhook.URL, SecretToken: webhook.Secret,
		AllowedUpdates: []string{"my_chat_member"}, DropPendingUpdates: true,
	}, &accepted)
	if err != nil {
		return err
	}
	if !accepted {
		return protocolError()
	}
	return nil
}

// DeleteWebhook 删除机器人当前 Webhook 并丢弃积压 Update。
func (c *Client) DeleteWebhook(ctx context.Context, token string) error {
	var deleted bool
	err := c.call(ctx, token, "deleteWebhook", struct {
		DropPendingUpdates bool `json:"drop_pending_updates"`
	}{DropPendingUpdates: true}, &deleted)
	if err != nil {
		return err
	}
	if !deleted {
		return protocolError()
	}
	return nil
}

// call 调用单个 Bot API 方法并校验 Telegram 响应信封。
func (c *Client) call(ctx context.Context, token, method string, input, output any) error {
	endpoint, err := c.methodURL(token, method)
	if err != nil {
		return connectiontest.InvalidConfigError(errors.New("invalid Telegram bot token"))
	}
	body, err := json.Marshal(input)
	if err != nil {
		return protocolError()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return connectiontest.InvalidConfigError(errors.New("invalid Telegram request"))
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return safeTransportError(err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return connectiontest.HTTPStatusError(response.StatusCode)
	}

	envelope := struct {
		OK        bool            `json:"ok"`
		Result    json.RawMessage `json:"result"`
		ErrorCode int             `json:"error_code"`
	}{}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseSize))
	if err := decoder.Decode(&envelope); err != nil {
		return protocolError()
	}
	if !envelope.OK {
		return telegramAPIError(envelope.ErrorCode)
	}
	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return protocolError()
	}
	if err := json.Unmarshal(envelope.Result, output); err != nil {
		return protocolError()
	}
	return nil
}

// methodURL 构造包含转义 Token 的 Bot API 方法地址。
func (c *Client) methodURL(token, method string) (string, error) {
	token = strings.TrimSpace(token)
	if !botTokenPattern.MatchString(token) {
		return "", errors.New("invalid token")
	}
	base, err := url.Parse(c.baseURL)
	if err != nil || !base.IsAbs() || base.Host == "" {
		return "", errors.New("invalid base URL")
	}
	base.RawPath = ""
	base.Path = strings.TrimRight(base.Path, "/") + "/bot" + token + "/" + method
	return base.String(), nil
}

// safeTransportError 保留错误分类但移除可能包含 Token URL 的原始错误。
func safeTransportError(err error) error {
	classified := connectiontest.ClassifyTransportError(connectiontest.StageConnect, err)
	stage, kind, ok := connectiontest.Details(classified)
	if !ok {
		return connectiontest.NewError(connectiontest.StageConnect, connectiontest.FailureUnavailable, nil)
	}
	return connectiontest.NewError(stage, kind, nil)
}

// telegramAPIError 按 Telegram error_code 生成安全的通用错误。
func telegramAPIError(code int) error {
	switch code {
	case http.StatusUnauthorized:
		return connectiontest.NewError(connectiontest.StageAuthenticate, connectiontest.FailureUnauthorized, nil)
	case http.StatusForbidden:
		return connectiontest.NewError(connectiontest.StageAuthorize, connectiontest.FailureForbidden, nil)
	case http.StatusTooManyRequests:
		return connectiontest.NewError(connectiontest.StageCapability, connectiontest.FailureRateLimited, nil)
	}
	if code >= http.StatusInternalServerError {
		return connectiontest.NewError(connectiontest.StageConnect, connectiontest.FailureUnavailable, nil)
	}
	return protocolError()
}

// protocolError 返回不包含 Telegram 原始响应的协议错误。
func protocolError() error {
	return connectiontest.NewError(connectiontest.StageCapability, connectiontest.FailureProtocol, nil)
}
