//go:build server

package inbox

import "errors"

// ErrQueryInvalid 表示收件箱范围或筛选条件无效。
var ErrQueryInvalid = errors.New("inbox query invalid")
