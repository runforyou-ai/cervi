// Package tenant 定义请求所属企业的解析边界。
package tenant

import (
	"context"
	"errors"
)

var (
	// ErrNotFound 表示当前请求没有匹配到企业。
	ErrNotFound = errors.New("tenant not found")
	// ErrAmbiguous 表示当前请求无法唯一匹配企业。
	ErrAmbiguous = errors.New("tenant is ambiguous")
)

// Scope 表示一次请求已经解析出的企业范围。
type Scope struct {
	OrganizationID   string
	OrganizationName string
}

// Resolver 根据规范化域名解析请求所属企业。
type Resolver interface {
	Resolve(context.Context, string) (Scope, error)
}
