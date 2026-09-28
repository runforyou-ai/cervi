//go:build server

package appservice

import (
	"time"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// MemberSession 定义实时网关持有的已认证成员会话：公开受众与存活时限所需的工作区、用户、登录会话编号与到期时间，身份详情只供后端内部使用。
type MemberSession struct {
	OrganizationID string
	UserID         string
	SessionID      string
	ExpiresAt      time.Time
	identity       *servermodels.Identity
}

// NewMemberSession 由已解析的成员身份创建成员会话。
func NewMemberSession(identity *servermodels.Identity) MemberSession {
	return MemberSession{
		OrganizationID: identity.Organization.ID, UserID: identity.User.ID,
		SessionID: identity.Session.ID, ExpiresAt: identity.Session.ExpiresAt, identity: identity,
	}
}
