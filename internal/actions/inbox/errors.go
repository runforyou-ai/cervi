//go:build server

package inbox

import "errors"

// ErrQueryInvalid 表示收件箱范围或筛选条件无效。
var ErrQueryInvalid = errors.New("inbox query invalid")

// ErrCursorInvalid 表示分页游标失效或不属于当前查询。
var ErrCursorInvalid = errors.New("inbox cursor invalid")
