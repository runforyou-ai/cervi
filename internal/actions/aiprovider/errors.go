//go:build server

package aiprovider

import "errors"

// ErrNotFound 表示当前企业中不存在指定 AI 供应商。
var ErrNotFound = errors.New("AI provider not found")
