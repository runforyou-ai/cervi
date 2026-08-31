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
	assignees := make([]CustomerServiceAssignee, 0)
	err := q.db.NewSelect().TableExpr("organization_identities AS oi").
		ColumnExpr("oi.id::text AS identity_id, oi.type, oi.display_name, oi.avatar_file_id::text").
		Join("JOIN roles AS r ON r.id = oi.role_id AND r.organization_id = oi.organization_id AND r.kind = ?", domain.RoleKindCustomerService).
		Join("LEFT JOIN users AS u ON u.identity_id = oi.id AND u.organization_id = oi.organization_id").
		Join("LEFT JOIN agents AS a ON a.identity_id = oi.id AND a.organization_id = oi.organization_id").
		Where("oi.organization_id = ?", identity.Organization.ID).
		Where("((oi.type = ? AND u.status = ?) OR (oi.type = ? AND a.status = ?))", domain.OrganizationIdentityTypeUser, domain.UserStatusActive, domain.OrganizationIdentityTypeAgent, domain.UserStatusActive).
		OrderExpr("lower(oi.display_name) ASC, oi.id ASC").
		Scan(ctx, &assignees)
	if err != nil {
		return nil, fmt.Errorf("list customer service assignees: %w", err)
	}
	return assignees, nil
}
