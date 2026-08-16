//go:build server

package models

// Principal 表示当前用户及其所属企业。
type Principal struct {
	Organization Organization `json:"organization"`
	User         User         `json:"user"`
}
