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
	// ErrLastActiveAdministrator 表示企业至少需要保留一名正常状态的管理员。
	ErrLastActiveAdministrator = errors.New("organization requires an active administrator")
	// ErrRoleChangesInvalid 表示批量角色调整参数无效。
	ErrRoleChangesInvalid = errors.New("user role changes invalid")
)
