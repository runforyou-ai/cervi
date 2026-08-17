//go:build server

package contact

import "errors"

var (
	// ErrNotFound 表示当前企业中不存在指定联系人。
	ErrNotFound = errors.New("contact not found")
	// ErrPrincipalInvalid 表示当前用户与企业关联已经失效。
	ErrPrincipalInvalid = errors.New("contact principal association is invalid")
)
