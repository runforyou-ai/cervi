//go:build server

package channel

import "errors"

var (
	// ErrNotFound 表示当前企业中不存在指定消息渠道。
	ErrNotFound = errors.New("message channel not found")
)
