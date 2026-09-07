// Package telegram 适配私聊文本发送及平台受理结果。
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"
)

// SendError 保留可安全记录的发送分类和平台等待时间。
type SendError struct {
	Code       string
	RetryAfter time.Duration
}

// Error 返回不含 Token 和平台响应正文的错误码。
func (e *SendError) Error() string { return e.Code }

// TextSender 定义私聊文本投递依赖。
type TextSender interface {
	SendText(context.Context, string, string, string) (int64, error)
}

// SendText 发送纯文本并保留平台拒绝与结果未知的区别。
func (c *Client) SendText(ctx context.Context, token, chatID, body string) (int64, error) {
	if !botTokenPattern.MatchString(token) {
		return 0, &SendError{Code: "invalid_token"}
	}
	chat, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil || chat <= 0 {
		return 0, &SendError{Code: "invalid_recipient"}
	}
	payload, err := json.Marshal(struct {
		ChatID int64  `json:"chat_id"`
		Text   string `json:"text"`
	}{chat, body})
	if err != nil {
		return 0, &SendError{Code: "invalid_message"}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bot"+token+"/sendMessage", bytes.NewReader(payload))
	if err != nil {
		return 0, &SendError{Code: "invalid_token"}
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return 0, &SendError{Code: "unknown_result"}
	}
	defer response.Body.Close()
	var envelope struct {
		OK         bool `json:"ok"`
		ErrorCode  int  `json:"error_code"`
		Parameters struct {
			RetryAfter int64 `json:"retry_after"`
		} `json:"parameters"`
		Result struct {
			MessageID int64 `json:"message_id"`
			Chat      struct {
				ID int64 `json:"id"`
			} `json:"chat"`
		} `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseSize)).Decode(&envelope); err != nil {
		return 0, &SendError{Code: "unknown_result"}
	}
	if response.StatusCode >= 500 {
		return 0, &SendError{Code: "unknown_result"}
	}
	if !envelope.OK {
		switch envelope.ErrorCode {
		case 429:
			if envelope.Parameters.RetryAfter > 0 && envelope.Parameters.RetryAfter <= int64((1<<63-1)/time.Second) {
				return 0, &SendError{Code: "rate_limited", RetryAfter: time.Duration(envelope.Parameters.RetryAfter) * time.Second}
			}
		case 401:
			return 0, &SendError{Code: "invalid_token"}
		case 403:
			return 0, &SendError{Code: "recipient_unavailable"}
		case 400:
			return 0, &SendError{Code: "message_rejected"}
		}
		return 0, &SendError{Code: "unknown_result"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || envelope.Result.MessageID <= 0 || envelope.Result.Chat.ID != chat {
		return 0, &SendError{Code: "unknown_result"}
	}
	return envelope.Result.MessageID, nil
}
