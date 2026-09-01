//go:build server

package inbox

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// CustomerServiceAssignee 定义可用于客服筛选的企业身份。
type CustomerServiceAssignee struct {
	IdentityID   string                          `bun:"identity_id"`
	Type         domain.OrganizationIdentityType `bun:"type"`
	DisplayName  string                          `bun:"display_name"`
	AvatarFileID *string                         `bun:"avatar_file_id"`
}

// ListCustomerServiceAssigneesQuery 读取有效客服身份。
type ListCustomerServiceAssigneesQuery struct{ db *bun.DB }

// NewListCustomerServiceAssigneesQuery 创建有效客服身份查询。
func NewListCustomerServiceAssigneesQuery(db *bun.DB) *ListCustomerServiceAssigneesQuery {
	return &ListCustomerServiceAssigneesQuery{db: db}
}

// Execute 返回角色为客服且账号有效的真人和 AI 员工。
func (q *ListCustomerServiceAssigneesQuery) Execute(ctx context.Context, identity *servermodels.Identity) ([]CustomerServiceAssignee, error) {
	if err := identityaction.Validate(ctx, q.db, identity); err != nil {
		return nil, err
	}
	identities, err := identityaction.ListActiveCustomerServiceIdentities(ctx, q.db, identity.Organization.ID)
	if err != nil {
		return nil, fmt.Errorf("list customer service assignees: %w", err)
	}
	assignees := make([]CustomerServiceAssignee, 0, len(identities))
	for _, item := range identities {
		assignees = append(assignees, CustomerServiceAssignee{
			IdentityID: item.ID, Type: domain.OrganizationIdentityType(item.Type),
			DisplayName: item.DisplayName, AvatarFileID: item.AvatarFileID,
		})
	}
	return assignees, nil
}
