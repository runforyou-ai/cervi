//go:build server

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
)

const telegramWebhookBodyLimit = 64 << 10

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
	memberPayload := bytes.TrimSpace(update.MyChatMember)
	myChatMember := len(memberPayload) > 0 && string(memberPayload) != "null"
	if myChatMember && memberPayload[0] != '{' {
		c.Status(http.StatusBadRequest)
		return
	}
	err := s.telegramWebhook.Execute(c.Request.Context(), channelID, channelaction.TelegramWebhookInput{
		Secret: secret, MyChatMember: myChatMember,
	})
	if writeTelegramWebhookError(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
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
	case errors.Is(err, channelaction.ErrTelegramWebhookUnsupported):
		c.Status(http.StatusServiceUnavailable)
	default:
		if c.Request.Context().Err() == nil {
			c.Status(http.StatusServiceUnavailable)
		}
	}
	return true
}
