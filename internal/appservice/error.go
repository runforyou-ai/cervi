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
	Kind    ErrorKind         `json:"kind,omitempty"`
	State   SessionState      `json:"state,omitempty"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	status  int               `json:"-"`
}

// Error 返回错误种类或会话状态，以及用户文案。
func (e *Error) Error() string {
	if e.Kind != "" {
		return string(e.Kind) + ": " + e.Message
	}
	if e.State != "" {
		return string(e.State) + ": " + e.Message
	}
	return e.Message
}

// HTTPStatus 返回对应的 HTTP 状态码。
func (e *Error) HTTPStatus() int {
	if e.status != 0 {
		return e.status
	}
	switch e.State {
	case SessionStateLogin:
		return http.StatusUnauthorized
	case SessionStateSetup:
		return http.StatusConflict
	case SessionStateConnect:
		return http.StatusPreconditionRequired
	}
	switch e.Kind {
	case ErrorKindInvalid:
		return http.StatusBadRequest
	case ErrorKindNotFound:
		return http.StatusNotFound
	case ErrorKindUnavailable:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// WithStatus 指定 HTTP 状态码。
func (e *Error) WithStatus(status int) *Error {
	e.status = status
	return e
}

// WithState 指定应进入的会话入口。
func (e *Error) WithState(state SessionState) *Error {
	e.State = state
	return e
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

// InvalidError 返回输入无效的业务错误。
func InvalidError(meta RequestMeta, messageKey cervii18n.Key, fieldKeys map[string]cervii18n.Key) *Error {
	return newError(meta, ErrorKindInvalid, "", messageKey, fieldKeys)
}

// NotFoundError 返回资源不存在的业务错误。
func NotFoundError(meta RequestMeta, messageKey cervii18n.Key) *Error {
	return newError(meta, ErrorKindNotFound, "", messageKey, nil)
}

// UnavailableError 返回依赖服务不可用的业务错误。
func UnavailableError(meta RequestMeta, messageKey cervii18n.Key, fieldKeys map[string]cervii18n.Key) *Error {
	return newError(meta, ErrorKindUnavailable, "", messageKey, fieldKeys)
}

// FailedError 返回操作失败的业务错误。
func FailedError(meta RequestMeta, messageKey cervii18n.Key) *Error {
	return newError(meta, ErrorKindFailed, "", messageKey, nil)
}

// SessionError 返回应回到会话入口的业务错误。
func SessionError(meta RequestMeta, state SessionState, messageKey cervii18n.Key) *Error {
	return newError(meta, "", state, messageKey, nil)
}

// SessionStateOf 读取错误中的会话入口。
func SessionStateOf(err error) SessionState {
	var apiError *Error
	if errors.As(err, &apiError) {
		return apiError.State
	}
	return ""
}

// methodNotAllowedError 返回当前平台不支持该操作的业务错误。
func methodNotAllowedError(meta RequestMeta, operation string) *Error {
	slog.Warn("当前平台不支持此操作", "operation", operation)
	return FailedError(meta, cervii18n.ErrorMethodNotAllowed).WithStatus(http.StatusMethodNotAllowed)
}

// newError 构造本地化业务错误。
func newError(meta RequestMeta, kind ErrorKind, state SessionState, messageKey cervii18n.Key, fieldKeys map[string]cervii18n.Key) *Error {
	message, _ := cervii18n.Localize(string(meta.Locale), messageKey)
	return &Error{Kind: kind, State: state, Message: message, Fields: cervii18n.LocalizeMap(string(meta.Locale), fieldKeys)}
}
