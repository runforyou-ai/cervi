//go:build server

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
)

// TestTelegramWebhook 验证公开回调认证、请求体限制和状态码契约。
func TestTelegramWebhook(t *testing.T) {
	tests := []struct {
		name          string
		secret        string
		body          string
		preflightErr  error
		executeErr    error
		status        int
		executeCalled bool
		myChatMember  bool
		message       *channelaction.TelegramWebhookMessage
	}{
		{name: "not found before body", secret: "secret", body: `{`, preflightErr: channelaction.ErrNotFound, status: http.StatusNotFound},
		{name: "unauthorized before body", secret: "wrong", body: `{`, preflightErr: channelaction.ErrTelegramWebhookUnauthorized, status: http.StatusUnauthorized},
		{name: "malformed body", secret: "secret", body: `{`, status: http.StatusBadRequest},
		{name: "missing update id", secret: "secret", body: `{"my_chat_member":{}}`, status: http.StatusBadRequest},
		{name: "trailing JSON", secret: "secret", body: `{"update_id":1,"my_chat_member":{}} {}`, status: http.StatusBadRequest},
		{name: "invalid member update", secret: "secret", body: `{"update_id":1,"my_chat_member":true}`, status: http.StatusBadRequest},
		{name: "multiple update types", secret: "secret", body: `{"update_id":1,"my_chat_member":{},"message":{}}`, status: http.StatusBadRequest},
		{name: "unknown update", secret: "secret", body: `{"update_id":1,"callback_query":{}}`, status: http.StatusNoContent, executeCalled: true},
		{name: "ignored group message", secret: "secret", body: `{"update_id":1,"message":{"message_id":2,"date":1725000000,"chat":{"id":-1,"type":"group"}}}`, status: http.StatusNoContent, executeCalled: true},
		{name: "ignored private non-text", secret: "secret", body: `{"update_id":1,"message":{"message_id":2,"date":1725000000,"chat":{"id":3,"type":"private"},"from":{"id":3,"first_name":"Cervi"}}}`, status: http.StatusNoContent, executeCalled: true},
		{name: "ignored cursor overflow date", secret: "secret", body: `{"update_id":1,"message":{"message_id":2,"date":9223372037,"text":"hello","chat":{"id":3,"type":"private"},"from":{"id":3,"first_name":"Cervi"}}}`, status: http.StatusNoContent, executeCalled: true},
		{name: "temporary execute failure", secret: "secret", body: `{"update_id":1,"callback_query":{}}`, executeErr: errors.New("database unavailable"), status: http.StatusServiceUnavailable, executeCalled: true},
		{
			name: "valid private text", secret: "secret",
			body:   `{"update_id":1,"message":{"message_id":2,"date":1725000000,"text":"  hello  ","chat":{"id":3,"type":"private"},"from":{"id":3,"first_name":"Cervi","last_name":"User"}}}`,
			status: http.StatusNoContent, executeCalled: true,
			message: &channelaction.TelegramWebhookMessage{
				ChatID: 3, MessageID: 2, SenderID: 3, DisplayName: "Cervi User",
				Body: "hello", OriginatedAt: time.Unix(1725000000, 0).UTC(),
			},
		},
		{name: "valid callback", secret: "secret", body: `{"update_id":1,"my_chat_member":{}}`, status: http.StatusNoContent, executeCalled: true, myChatMember: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			receiver := &telegramWebhookReceiverStub{
				preflightErr: test.preflightErr,
				executeErr:   test.executeErr,
			}
			service := NewService(nil, WithTelegramWebhook(receiver))
			request := httptest.NewRequest(
				http.MethodPost,
				"/public/telegram-channels/channel-id/webhook",
				strings.NewReader(test.body),
			)
			request.Header.Set("X-Telegram-Bot-Api-Secret-Token", test.secret)
			response := httptest.NewRecorder()

			service.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			if receiver.executeCalled != test.executeCalled {
				t.Fatalf("execute called = %t, want %t", receiver.executeCalled, test.executeCalled)
			}
			if receiver.executeCalled && receiver.input.MyChatMember != test.myChatMember {
				t.Fatalf("my_chat_member = %t, want %t", receiver.input.MyChatMember, test.myChatMember)
			}
			if receiver.executeCalled && !telegramWebhookMessagesEqual(receiver.input.Message, test.message) {
				t.Fatalf("message = %#v, want %#v", receiver.input.Message, test.message)
			}
			if receiver.secret != test.secret {
				t.Fatalf("preflight secret = %q, want %q", receiver.secret, test.secret)
			}
			if response.Body.Len() != 0 {
				t.Fatalf("response body = %q, want empty", response.Body.String())
			}
		})
	}
}

// TestTelegramWebhookRejectsOversizedBody 验证超过 64 KiB 的回调体被拒绝。
func TestTelegramWebhookRejectsOversizedBody(t *testing.T) {
	receiver := &telegramWebhookReceiverStub{}
	service := NewService(nil, WithTelegramWebhook(receiver))
	body := `{"update_id":1,"my_chat_member":{"value":"` + strings.Repeat("x", telegramWebhookBodyLimit) + `"}}`
	request := httptest.NewRequest(http.MethodPost, "/public/telegram-channels/channel-id/webhook", strings.NewReader(body))
	request.Header.Set("X-Telegram-Bot-Api-Secret-Token", "secret")
	response := httptest.NewRecorder()

	service.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if receiver.executeCalled {
		t.Fatal("oversized body reached action")
	}
}

type telegramWebhookReceiverStub struct {
	preflightErr  error
	executeErr    error
	secret        string
	input         channelaction.TelegramWebhookInput
	executeCalled bool
}

// Preflight 记录测试请求头并返回预设错误。
func (s *telegramWebhookReceiverStub) Preflight(_ context.Context, _ string, secret string) error {
	s.secret = secret
	return s.preflightErr
}

// Execute 记录解析后的回调输入并返回预设错误。
func (s *telegramWebhookReceiverStub) Execute(_ context.Context, _ string, input channelaction.TelegramWebhookInput) error {
	s.executeCalled = true
	s.input = input
	return s.executeErr
}

// telegramWebhookMessagesEqual 比较回调归一化消息。
func telegramWebhookMessagesEqual(left, right *channelaction.TelegramWebhookMessage) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.ChatID == right.ChatID && left.MessageID == right.MessageID && left.SenderID == right.SenderID &&
		left.DisplayName == right.DisplayName && left.Body == right.Body && left.OriginatedAt.Equal(right.OriginatedAt)
}

var _ TelegramWebhookReceiver = (*telegramWebhookReceiverStub)(nil)

// TestTelegramWebhookReplyNormalization 验证机器人原文、跨聊天引用与部分引用的整条语义。
func TestTelegramWebhookReplyNormalization(t *testing.T) {
	for _, test := range []struct {
		name string
		chat int64
		want bool
	}{
		{"same chat bot reply", 123, true}, {"another chat", 456, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var message telegramWebhookMessage
			body := fmt.Sprintf(`{"message_id":9,"date":1788880000,"text":"客户回答","chat":{"id":123,"type":"private"},"from":{"id":123,"first_name":"客户"},"quote":{"text":"部分"},"reply_to_message":{"message_id":7,"text":"完整原文","chat":{"id":%d,"type":"private"},"from":{"id":999,"is_bot":true,"first_name":"客服 Bot"}}}`, test.chat)
			if err := json.Unmarshal([]byte(body), &message); err != nil {
				t.Fatal(err)
			}
			result, reason := normalizeTelegramWebhookMessage(message)
			if result == nil || reason != "" || (result.Reply != nil) != test.want {
				t.Fatalf("result=%+v reason=%s", result, reason)
			}
			if test.want && (result.Reply.MessageID != 7 || result.Reply.Body != "完整原文" || !result.Reply.SenderIsBot || result.Reply.SenderName != "客服 Bot") {
				t.Fatalf("reply=%+v", result.Reply)
			}
		})
	}
}

// TestTelegramWebhookMediaNormalization 验证七类媒体的解析、默认文件名与类型、照片档位选择和说明校验。
func TestTelegramWebhookMediaNormalization(t *testing.T) {
	envelope := func(payload string) string {
		return `{"message_id":9,"date":1788880000,"chat":{"id":123,"type":"private"},"from":{"id":123,"first_name":"客户"},` + payload + `}`
	}
	for _, test := range []struct {
		name   string
		body   string
		reason string
		media  *channelaction.TelegramWebhookMedia
		text   string
	}{
		{
			name: "photo picks largest size", body: envelope(`"caption":" 看图 ","photo":[{"file_id":"small","file_unique_id":"u1","width":90,"height":60,"file_size":100},{"file_id":"large","file_unique_id":"u1","width":800,"height":600,"file_size":90000},{"file_id":"broken","file_unique_id":"u1","width":0,"height":0}]`),
			media: &channelaction.TelegramWebhookMedia{FileID: "large", UniqueID: "u1", FileName: "photo.jpg", ContentType: "image/jpeg", ByteSize: 90000, Width: 800, Height: 600}, text: "看图",
		},
		{
			name: "document keeps name and type", body: envelope(`"document":{"file_id":"doc","file_unique_id":"u2","file_name":"合同.pdf","mime_type":"application/pdf","file_size":2048}`),
			media: &channelaction.TelegramWebhookMedia{FileID: "doc", UniqueID: "u2", FileName: "合同.pdf", ContentType: "application/pdf", ByteSize: 2048},
		},
		{
			name: "animation wins over document", body: envelope(`"document":{"file_id":"doc","file_unique_id":"u3","file_name":"x.gif.mp4","mime_type":"video/mp4"},"animation":{"file_id":"anim","file_unique_id":"u3","width":320,"height":240,"file_size":500}`),
			media: &channelaction.TelegramWebhookMedia{FileID: "anim", UniqueID: "u3", FileName: "animation.mp4", ContentType: "video/mp4", ByteSize: 500},
		},
		{
			name: "voice default name", body: envelope(`"voice":{"file_id":"voice","file_unique_id":"u4","mime_type":"audio/ogg","file_size":7}`),
			media: &channelaction.TelegramWebhookMedia{FileID: "voice", UniqueID: "u4", FileName: "voice.ogg", ContentType: "audio/ogg", ByteSize: 7},
		},
		{
			name: "video note default type", body: envelope(`"video_note":{"file_id":"note","file_unique_id":"u5","length":240,"file_size":9}`),
			media: &channelaction.TelegramWebhookMedia{FileID: "note", UniqueID: "u5", FileName: "video-note.mp4", ContentType: "video/mp4", ByteSize: 9},
		},
		{
			name: "video dimensions are not image size", body: envelope(`"video":{"file_id":"video","file_unique_id":"u6","width":1920,"height":1080,"mime_type":"video/mp4","file_name":"clip.mp4"}`),
			media: &channelaction.TelegramWebhookMedia{FileID: "video", UniqueID: "u6", FileName: "clip.mp4", ContentType: "video/mp4"},
		},
		{
			name: "audio default name", body: envelope(`"audio":{"file_id":"audio","file_unique_id":"u7","mime_type":"audio/mpeg","file_size":11}`),
			media: &channelaction.TelegramWebhookMedia{FileID: "audio", UniqueID: "u7", FileName: "audio.mp3", ContentType: "audio/mpeg", ByteSize: 11},
		},
		{name: "sticker unsupported", body: envelope(`"sticker":{"file_id":"sticker","file_unique_id":"u8"}`), reason: "unsupported_content"},
		{name: "missing unique id", body: envelope(`"document":{"file_id":"doc"}`), reason: "invalid_media"},
		{name: "caption too long", body: envelope(`"caption":"` + strings.Repeat("长", 1025) + `","document":{"file_id":"doc","file_unique_id":"u9"}`), reason: "invalid_caption"},
		{name: "text ignores media", body: envelope(`"text":"你好","photo":[{"file_id":"p","file_unique_id":"u10","width":1,"height":1}]`), text: "你好"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var message telegramWebhookMessage
			if err := json.Unmarshal([]byte(test.body), &message); err != nil {
				t.Fatal(err)
			}
			result, reason := normalizeTelegramWebhookMessage(message)
			if reason != test.reason {
				t.Fatalf("reason=%q want %q", reason, test.reason)
			}
			if test.reason != "" {
				return
			}
			if result.Body != test.text {
				t.Fatalf("body=%q want %q", result.Body, test.text)
			}
			if (result.Media == nil) != (test.media == nil) || (test.media != nil && *result.Media != *test.media) {
				t.Fatalf("media=%+v want %+v", result.Media, test.media)
			}
		})
	}
}
