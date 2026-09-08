//go:build server

package mcpserver

import "errors"

var (
	// ErrNotFound 表示当前企业中不存在指定 MCP 服务。
	ErrNotFound = errors.New("MCP server not found")
)
