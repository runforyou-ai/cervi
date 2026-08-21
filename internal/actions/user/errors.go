//go:build server

package user

import "errors"

var (
	// ErrNotFound 表示当前企业中不存在指定成员。
	ErrNotFound = errors.New("user not found")
	// ErrQueryInvalid 表示企业成员列表查询无效。
	ErrQueryInvalid = errors.New("user list query invalid")
	// ErrSelfDeactivate 表示用户不能停用当前登录账号。
	ErrSelfDeactivate = errors.New("current user cannot be deactivated")
)
