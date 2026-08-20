package common

import "errors"

// ErrIdentityInvalid 表示当前用户与企业关联已经失效。
var ErrIdentityInvalid = errors.New("identity association is invalid")
