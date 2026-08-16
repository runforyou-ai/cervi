//go:build server

// Package inbox 实现统一收件箱领域的应用查询。
package inbox

import (
	"context"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// LoadInboxQuery 读取当前用户的统一收件箱。
type LoadInboxQuery struct{}

// LoadInboxOutput 定义统一收件箱查询结果。
type LoadInboxOutput struct {
	Organization  servermodels.Organization
	User          servermodels.User
	Conversations []any
}

// NewLoadInboxQuery 创建统一收件箱查询。
func NewLoadInboxQuery() *LoadInboxQuery {
	return &LoadInboxQuery{}
}

// Execute 返回当前身份可访问的统一收件箱。
func (q *LoadInboxQuery) Execute(_ context.Context, principal *servermodels.Principal) LoadInboxOutput {
	return LoadInboxOutput{
		Organization:  principal.Organization,
		User:          principal.User,
		Conversations: []any{},
	}
}
