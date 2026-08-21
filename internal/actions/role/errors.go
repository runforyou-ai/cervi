//go:build server

package role

import "errors"

var (
	// ErrNotFound 表示当前企业中不存在指定角色。
	ErrNotFound = errors.New("role not found")
	// ErrAdminImmutable 表示管理员角色不可修改。
	ErrAdminImmutable = errors.New("administrator role is immutable")
	// ErrBuiltInDeleteForbidden 表示内置角色不可删除。
	ErrBuiltInDeleteForbidden = errors.New("built-in role cannot be deleted")
)
