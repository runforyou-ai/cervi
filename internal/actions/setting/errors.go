//go:build server

package setting

import "errors"

// ErrPrincipalInvalid 表示当前用户与企业关联已经失效。
var ErrPrincipalInvalid = errors.New("setting principal association is invalid")
