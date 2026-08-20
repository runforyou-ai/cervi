//go:build server

package channel

import "errors"

var (
	// ErrNotFound 表示当前企业中不存在指定网站渠道。
	ErrNotFound = errors.New("website channel not found")
)
