//go:build server

package integrationconnection

import "errors"

var (
	// ErrNotFound 表示当前企业中不存在指定连接器。
	ErrNotFound = errors.New("integration connection not found")
	// ErrInUse 表示连接器仍被业务数据使用。
	ErrInUse = errors.New("integration connection is in use")
)
