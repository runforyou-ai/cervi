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
	"github.com/runforyou-ai/cervi/internal/domain"
)

const telegramWebhookBodyLimit = 64 << 10

const telegramTextLimit = 4096

const telegramMaxCursorUnixSecond = int64(9223372036)

const (
	telegramMediaFileIDLimit   = 512
	telegramMediaFileNameLimit = 255
)

// telegramWebhookFile 定义 Telegram 各类媒体共有的文件引用与元数据。
type telegramWebhookFile struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileName     string `json:"file_name"`
	MimeType     string `json:"mime_type"`
	FileSize     int64  `json:"file_size"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

type telegramWebhookMessage struct {
	ReplyTo   *telegramWebhookMessage `json:"reply_to_message"`
	Photo     []telegramWebhookFile   `json:"photo"`
	Document  *telegramWebhookFile    `json:"document"`
	Voice     *telegramWebhookFile    `json:"voice"`
	Video     *telegramWebhookFile    `json:"video"`
	VideoNote *telegramWebhookFile    `json:"video_note"`
	Audio     *telegramWebhookFile    `json:"audio"`
	Animation *telegramWebhookFile    `json:"animation"`
	Caption   string                  `json:"caption"`
	MessageID int64                   `json:"message_id"`
	Date      int64                   `json:"date"`
	Text      *string                 `json:"text"`
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
	// 拒绝一个请求体中的多段 JSON。
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
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
	var body string
	var media *channelaction.TelegramWebhookMedia
	if message.Text != nil {
		body = strings.TrimSpace(*message.Text)
		if body == "" || !utf8.ValidString(body) || utf8.RuneCountInString(body) > telegramTextLimit {
			return nil, "invalid_text"
		}
	} else {
		var ignoredReason string
		media, ignoredReason = normalizeTelegramWebhookMedia(message)
		if ignoredReason != "" {
			return nil, ignoredReason
		}
		body = strings.TrimSpace(message.Caption)
		if !utf8.ValidString(body) || utf8.RuneCountInString(body) > domain.ChannelCaptionLimit(domain.ChannelTypeTelegram) {
			return nil, "invalid_caption"
		}
	}
	displayName := strings.TrimSpace(strings.Join([]string{message.From.FirstName, message.From.LastName}, " "))
	if displayName == "" {
		return nil, "missing_sender_name"
	}
	var reply *channelaction.TelegramWebhookReply
	// 原消息可由机器人发送，只读取同一聊天中的一层引用。
	if original := message.ReplyTo; original != nil && original.Chat.ID == message.Chat.ID && original.MessageID > 0 && original.MessageID != message.MessageID {
		reply = &channelaction.TelegramWebhookReply{MessageID: original.MessageID, Body: original.Caption}
		if original.Text != nil {
			reply.Body = *original.Text
		}
		if original.From != nil {
			reply.SenderName = strings.TrimSpace(strings.Join([]string{original.From.FirstName, original.From.LastName}, " "))
			reply.SenderIsBot = original.From.IsBot
		}
	}
	originatedAt := time.Unix(message.Date, 0).UTC()
	return &channelaction.TelegramWebhookMessage{
		ChatID: message.Chat.ID, MessageID: message.MessageID, Reply: reply, Media: media,
		SenderID: message.From.ID, DisplayName: displayName,
		Body: body, OriginatedAt: originatedAt,
	}, ""
}

// normalizeTelegramWebhookMedia 按动画、文件、照片、语音、视频、视频留言、音乐的顺序取出消息携带的单个媒体，缺少文件名或类型时按种类补默认值。
func normalizeTelegramWebhookMedia(message telegramWebhookMessage) (*channelaction.TelegramWebhookMedia, string) {
	var file *telegramWebhookFile
	var defaultName, defaultType string
	switch {
	case message.Animation != nil:
		// 动画消息同时携带 document 字段，按动画处理。
		file, defaultName, defaultType = message.Animation, "animation.mp4", "video/mp4"
	case message.Document != nil:
		file, defaultName, defaultType = message.Document, "document", "application/octet-stream"
	case len(message.Photo) > 0:
		// 照片取像素面积最大的档位。
		for index := range message.Photo {
			size := &message.Photo[index]
			if size.Width <= 0 || size.Height <= 0 {
				continue
			}
			if file == nil || size.Width*size.Height > file.Width*file.Height {
				file = size
			}
		}
		defaultName, defaultType = "photo.jpg", "image/jpeg"
	case message.Voice != nil:
		file, defaultName, defaultType = message.Voice, "voice.ogg", "audio/ogg"
	case message.Video != nil:
		file, defaultName, defaultType = message.Video, "video.mp4", "video/mp4"
	case message.VideoNote != nil:
		file, defaultName, defaultType = message.VideoNote, "video-note.mp4", "video/mp4"
	case message.Audio != nil:
		file, defaultName, defaultType = message.Audio, "audio.mp3", "audio/mpeg"
	default:
		return nil, "unsupported_content"
	}
	if file == nil || file.FileID == "" || len(file.FileID) > telegramMediaFileIDLimit || file.FileUniqueID == "" || file.FileSize < 0 || file.Width < 0 || file.Height < 0 {
		return nil, "invalid_media"
	}
	media := &channelaction.TelegramWebhookMedia{
		FileID: file.FileID, UniqueID: file.FileUniqueID, ByteSize: file.FileSize,
		FileName: strings.TrimSpace(file.FileName), ContentType: strings.TrimSpace(file.MimeType),
	}
	if media.FileName == "" || !utf8.ValidString(media.FileName) || utf8.RuneCountInString(media.FileName) > telegramMediaFileNameLimit {
		media.FileName = defaultName
	}
	if media.ContentType == "" {
		media.ContentType = defaultType
	}
	// 只有照片按图片尺寸内联展示，视频和动画的尺寸不作为图片宽高。
	if len(message.Photo) > 0 && message.Animation == nil && message.Document == nil {
		media.Width, media.Height = file.Width, file.Height
	}
	return media, ""
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
