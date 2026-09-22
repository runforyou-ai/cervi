//go:build server

package organization

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/uptrace/bun"
)

// ErrNotFound 表示目标企业不存在。
var ErrNotFound = errors.New("organization not found")

// Entitlement 描述企业当前生效的服务权益快照。
type Entitlement struct {
	Revision      int64
	PlanCode      string
	ServiceEndsAt *time.Time
	AppliedAt     time.Time
}

// Summary 描述运营侧查看的企业摘要；Entitlement 与 ProvisioningID 只在企业经运营开通时存在。
type Summary struct {
	ID              string
	Name            string
	AccessHost      string
	LifecycleStatus domain.OrganizationLifecycleStatus
	ProvisioningID  *string
	Entitlement     *Entitlement
	CreatedAt       time.Time
}

// ListSummariesInput 定义运营企业列表的筛选与分页条件，LifecycleStatus 为空表示不按状态筛选。
type ListSummariesInput struct {
	Query           string
	LifecycleStatus domain.OrganizationLifecycleStatus
	Page            int
	PageSize        int
}

// SummaryList 是一页企业摘要及符合条件的总数。
type SummaryList struct {
	Items []Summary
	Total int
}

// summaryRow 是企业摘要查询的扫描行。
type summaryRow struct {
	ID              string
	Name            string
	AccessHost      string
	LifecycleStatus string
	ProvisioningID  *string
	Revision        *int64
	PlanCode        *string
	ServiceEndsAt   *time.Time
	AppliedAt       *time.Time
	CreatedAt       time.Time
}

// SummaryQuery 跨企业读取运营侧企业摘要。
type SummaryQuery struct {
	db *bun.DB
}

// NewSummaryQuery 创建运营侧企业摘要查询。
func NewSummaryQuery(db *bun.DB) *SummaryQuery {
	return &SummaryQuery{db: db}
}

// Get 返回指定企业的摘要，企业不存在时返回 ErrNotFound。
func (q *SummaryQuery) Get(ctx context.Context, organizationID string) (Summary, error) {
	organizationID, ok := common.NormalizeUUID(strings.TrimSpace(organizationID))
	if !ok {
		return Summary{}, ErrNotFound
	}
	var row summaryRow
	err := q.selectSummaries().Where("o.id = ?", organizationID).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return Summary{}, ErrNotFound
	}
	if err != nil {
		return Summary{}, err
	}
	return row.summary(), nil
}

// List 按名称或访问地址关键字和生命周期状态分页返回企业摘要，按创建时间倒序。
func (q *SummaryQuery) List(ctx context.Context, input ListSummariesInput) (SummaryList, error) {
	query := q.selectSummaries()
	if keyword := strings.TrimSpace(input.Query); keyword != "" {
		pattern := "%" + keyword + "%"
		query = query.Where("(o.name ILIKE ? OR o.access_host ILIKE ?)", pattern, pattern)
	}
	if input.LifecycleStatus != "" {
		query = query.Where("o.lifecycle_status = ?", string(input.LifecycleStatus))
	}
	var rows []summaryRow
	total, err := query.OrderExpr("o.created_at DESC, o.id DESC").
		Limit(input.PageSize).
		Offset((input.Page-1)*input.PageSize).
		ScanAndCount(ctx, &rows)
	if err != nil {
		return SummaryList{}, err
	}
	items := make([]Summary, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.summary())
	}
	return SummaryList{Items: items, Total: total}, nil
}

// selectSummaries 构造关联开通记录和权益快照的企业摘要查询。
func (q *SummaryQuery) selectSummaries() *bun.SelectQuery {
	return q.db.NewSelect().
		TableExpr("organizations AS o").
		Join("LEFT JOIN operator_provisionings AS op ON op.organization_id = o.id").
		Join("LEFT JOIN organization_entitlements AS oe ON oe.organization_id = o.id").
		ColumnExpr("o.id::text, o.name, o.access_host, o.lifecycle_status, o.created_at, op.provisioning_id").
		ColumnExpr("oe.revision, oe.plan_code, oe.service_ends_at, oe.applied_at")
}

// summary 把扫描行转换为企业摘要。
func (row summaryRow) summary() Summary {
	summary := Summary{
		ID:              row.ID,
		Name:            row.Name,
		AccessHost:      row.AccessHost,
		LifecycleStatus: domain.OrganizationLifecycleStatus(row.LifecycleStatus),
		ProvisioningID:  row.ProvisioningID,
		CreatedAt:       row.CreatedAt,
	}
	// 企业有权益快照时组装权益摘要。
	if row.Revision != nil && row.PlanCode != nil && row.AppliedAt != nil {
		summary.Entitlement = &Entitlement{
			Revision:      *row.Revision,
			PlanCode:      *row.PlanCode,
			ServiceEndsAt: row.ServiceEndsAt,
			AppliedAt:     *row.AppliedAt,
		}
	}
	return summary
}
