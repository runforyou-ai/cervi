package appservice

import (
	"encoding/json"
	"errors"
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
	payload, _ := json.Marshal(apiError)
	return payload
}
