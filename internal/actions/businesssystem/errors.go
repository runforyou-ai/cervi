//go:build server

package businesssystem

import "errors"

var (
	// ErrNotFound 表示当前企业中不存在指定业务系统。
	ErrNotFound = errors.New("business system not found")
)
