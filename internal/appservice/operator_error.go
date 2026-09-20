//go:build server

package appservice

import (
	"errors"
	"net/http"

	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// OperatorErrorCode 是运营接口的稳定错误码，供 SaaS 按语义处理。
type OperatorErrorCode string

const (
	OperatorErrorCodeInvalidCredential OperatorErrorCode = "invalid_operator_credential"
	OperatorErrorCodeInvalidRequest    OperatorErrorCode = "invalid_request"
	OperatorErrorCodeInternal          OperatorErrorCode = "internal"
)

// OperatorError 定义运营接口的结构化错误。
type OperatorError struct {
	Code      OperatorErrorCode `json:"code"`
	Message   string            `json:"message"`
	RequestID string            `json:"requestId"`
	status    int
}

// Error 返回错误码和用户文案。
func (e *OperatorError) Error() string {
	return string(e.Code) + ": " + e.Message
}

// HTTPStatus 返回对应的 HTTP 状态码。
func (e *OperatorError) HTTPStatus() int {
	return e.status
}

// OperatorErrorOf 读取运营接口的结构化错误。
func OperatorErrorOf(err error) (*OperatorError, bool) {
	return errors.AsType[*OperatorError](err)
}

// NewOperatorInternalError 返回运营调用的内部失败，供 HTTP 适配层收敛未分类错误。
func NewOperatorInternalError(meta OperatorRequestMeta) *OperatorError {
	return newOperatorError(meta, http.StatusInternalServerError, OperatorErrorCodeInternal, cervii18n.ErrorInternal)
}

// NewOperatorInvalidRequestError 返回运营请求参数无效的错误，供 HTTP 适配层在参数绑定失败时使用。
func NewOperatorInvalidRequestError(meta OperatorRequestMeta) *OperatorError {
	return newOperatorError(meta, http.StatusBadRequest, OperatorErrorCodeInvalidRequest, cervii18n.ErrorValidationFailed)
}

// invalidOperatorCredentialError 返回运营凭据无效的错误。
func invalidOperatorCredentialError(meta OperatorRequestMeta) *OperatorError {
	return newOperatorError(meta, http.StatusUnauthorized, OperatorErrorCodeInvalidCredential, cervii18n.ErrorOperatorCredentialInvalid)
}

// newOperatorError 构造本地化的运营接口错误。
func newOperatorError(meta OperatorRequestMeta, status int, code OperatorErrorCode, messageKey cervii18n.Key) *OperatorError {
	message, _ := cervii18n.Localize(string(meta.Locale), messageKey)
	return &OperatorError{Code: code, Message: message, RequestID: meta.RequestID, status: status}
}
