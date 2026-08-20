// Package fielderror 定义 Action 共用的字段校验错误。
package fielderror

// Code 标识一条字段校验结果。
type Code string

// Error 记录字段校验失败。
type Error struct {
	Fields map[string]Code
}

// Error 返回字段校验失败说明。
func (e *Error) Error() string {
	return "validation failed"
}
