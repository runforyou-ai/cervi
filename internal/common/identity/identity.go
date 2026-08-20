// Package identity 提供当前登录身份的共用错误。
package identity

import "errors"

// ErrInvalid 表示当前用户与企业关联已经失效。
var ErrInvalid = errors.New("identity association is invalid")
