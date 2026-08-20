package appservice

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// Error 定义跨 Wails 和 HTTP 传输的业务错误。
type Error struct {
	Status  int               `json:"status"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// Error 返回错误码和用户文案。
func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}

// MarshalError 将业务错误写入 Wails RuntimeError 的 cause。
func MarshalError(err error) []byte {
	var apiError *Error
	if !errors.As(err, &apiError) {
		return nil
	}
	payload, err := json.Marshal(apiError)
	if err != nil {
		slog.Warn("序列化业务错误失败", "error", err)
		return nil
	}
	return payload
}

// methodNotAllowedError 返回当前平台不支持该操作的业务错误。
func methodNotAllowedError(meta RequestMeta, operation string) *Error {
	slog.Warn("当前平台不支持此操作", "operation", operation)
	message, _ := cervii18n.Localize(string(meta.Locale), cervii18n.ErrorMethodNotAllowed)
	return &Error{Status: http.StatusMethodNotAllowed, Code: "METHOD_NOT_ALLOWED", Message: message}
}
