//go:build server

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
)

const telegramWebhookBodyLimit = 64 << 10

const telegramTextLimit = 4096

const telegramMaxCursorUnixSecond = int64(9223372036)

type telegramWebhookMessage struct {
	MessageID int64   `json:"message_id"`
	Date      int64   `json:"date"`
	Text      *string `json:"text"`
	Chat      struct {
		ID   int64  `json:"id"`
		Type string `json:"type"`
	} `json:"chat"`
	From *struct {
		ID        int64  `json:"id"`
		IsBot     bool   `json:"is_bot"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	} `json:"from"`
}

// TelegramWebhookReceiver 定义公开回调使用的最小应用操作。
type TelegramWebhookReceiver interface {
	Preflight(context.Context, string, string) error
	Execute(context.Context, string, channelaction.TelegramWebhookInput) error
}

// registerTelegramWebhookRoutes 注册无需 Bearer Token 的 Telegram 回调。
func (s *Service) registerTelegramWebhookRoutes(router *gin.Engine) {
	if s.telegramWebhook == nil {
		return
	}
	router.POST("/public/telegram-channels/:channelID/webhook", s.receiveTelegramWebhook)
}

// receiveTelegramWebhook 认证 Telegram Update 并返回裸 HTTP 状态码。
func (s *Service) receiveTelegramWebhook(c *gin.Context) {
	channelID := c.Param("channelID")
	secret := c.GetHeader("X-Telegram-Bot-Api-Secret-Token")
	if writeTelegramWebhookError(c, s.telegramWebhook.Preflight(c.Request.Context(), channelID, secret)) {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, telegramWebhookBodyLimit)
	update := struct {
		UpdateID     *int64          `json:"update_id"`
		MyChatMember json.RawMessage `json:"my_chat_member"`
		Message      json.RawMessage `json:"message"`
	}{}
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(&update); err != nil || update.UpdateID == nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if err := ensureJSONEnd(decoder); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	myChatMember, validMember := telegramObjectPresent(update.MyChatMember)
	messagePresent, validMessage := telegramObjectPresent(update.Message)
	if !validMember || !validMessage || (myChatMember && messagePresent) {
		c.Status(http.StatusBadRequest)
		return
	}
	var message *channelaction.TelegramWebhookMessage
	if messagePresent {
		parsed := telegramWebhookMessage{}
		if err := json.Unmarshal(update.Message, &parsed); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		var ignoredReason string
		message, ignoredReason = normalizeTelegramWebhookMessage(parsed)
		if ignoredReason != "" {
			slog.Info("Telegram Update 已按范围忽略", "channel_id", channelID, "update_id", *update.UpdateID, "reason", ignoredReason)
		}
	}
	err := s.telegramWebhook.Execute(c.Request.Context(), channelID, channelaction.TelegramWebhookInput{
		Secret: secret, UpdateID: *update.UpdateID, MyChatMember: myChatMember, Message: message,
	})
	if writeTelegramWebhookError(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

// telegramObjectPresent 校验可选 Telegram Update 字段是否为对象。
func telegramObjectPresent(value json.RawMessage) (bool, bool) {
	payload := bytes.TrimSpace(value)
	if len(payload) == 0 || string(payload) == "null" {
		return false, true
	}
	return true, payload[0] == '{'
}

// normalizeTelegramWebhookMessage 归一化当前支持的私聊文本消息。
func normalizeTelegramWebhookMessage(message telegramWebhookMessage) (*channelaction.TelegramWebhookMessage, string) {
	if message.Chat.Type != "private" {
		return nil, "non_private"
	}
	if message.From == nil || message.From.IsBot || message.From.ID <= 0 || message.Chat.ID <= 0 || message.Chat.ID != message.From.ID || message.MessageID <= 0 || message.Date <= 0 || message.Date > telegramMaxCursorUnixSecond {
		return nil, "invalid_private_message"
	}
	if message.Text == nil {
		return nil, "non_text"
	}
	body := strings.TrimSpace(*message.Text)
	if body == "" || !utf8.ValidString(body) || utf8.RuneCountInString(body) > telegramTextLimit {
		return nil, "invalid_text"
	}
	displayName := strings.TrimSpace(strings.Join([]string{message.From.FirstName, message.From.LastName}, " "))
	if displayName == "" {
		return nil, "missing_sender_name"
	}
	originatedAt := time.Unix(message.Date, 0).UTC()
	return &channelaction.TelegramWebhookMessage{
		ChatID: message.Chat.ID, MessageID: message.MessageID,
		SenderID: message.From.ID, DisplayName: displayName,
		Body: body, OriginatedAt: originatedAt,
	}, ""
}

// ensureJSONEnd 拒绝一个请求体中的多段 JSON。
func ensureJSONEnd(decoder *json.Decoder) error {
	var trailing json.RawMessage
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("unexpected trailing JSON")
	}
	return err
}

// writeTelegramWebhookError 映射公开回调错误并返回是否已经响应。
func writeTelegramWebhookError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, channelaction.ErrNotFound):
		c.Status(http.StatusNotFound)
	case errors.Is(err, channelaction.ErrTelegramWebhookUnauthorized):
		c.Status(http.StatusUnauthorized)
	default:
		if c.Request.Context().Err() == nil {
			c.Status(http.StatusServiceUnavailable)
		}
	}
	return true
}
