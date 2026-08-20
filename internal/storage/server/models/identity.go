//go:build server

package models

// Identity 表示当前用户及其所属企业。
type Identity struct {
	Organization Organization `json:"organization"`
	User         User         `json:"user"`
}
