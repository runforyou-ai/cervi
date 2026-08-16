//go:build server

package channel

import "errors"

var (
	// ErrNotFound 表示当前企业中不存在指定网站渠道。
	ErrNotFound = errors.New("website channel not found")
	// ErrPrincipalInvalid 表示当前用户与企业关联已经失效。
	ErrPrincipalInvalid = errors.New("channel principal association is invalid")
)
