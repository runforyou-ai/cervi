//go:build server

package appservice

import (
	"context"
	"errors"
	"net/http"
	"time"

	organizationaction "github.com/runforyou-ai/cervi/internal/actions/organization"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// operatorOrganizationPageSizeMax 是运营工作区列表单页的最大条数。
const operatorOrganizationPageSizeMax = 100

// ListOrganizations 按条件分页返回工作区摘要。
func (o *operatorOperations) ListOrganizations(ctx context.Context, meta OperatorRequestMeta, _ OperatorIdentity, input OperatorOrganizationListInput) (OperatorOrganizationList, error) {
	if input.PageSize > operatorOrganizationPageSizeMax {
		return OperatorOrganizationList{}, NewOperatorInvalidRequestError(meta)
	}
	// 生命周期筛选只接受受支持的状态。
	if input.LifecycleStatus != nil {
		switch *input.LifecycleStatus {
		case OrganizationLifecycleActive, OrganizationLifecycleSuspended, OrganizationLifecycleDeleting, OrganizationLifecycleDeleted:
		default:
			return OperatorOrganizationList{}, NewOperatorInvalidRequestError(meta)
		}
	}
	list, err := o.organizationSummaries.List(ctx, organizationaction.ListSummariesInput{
		Query:           input.Query,
		LifecycleStatus: optionalDomain[OrganizationLifecycleStatus, domain.OrganizationLifecycleStatus](input.LifecycleStatus),
		Page:            input.Page,
		PageSize:        input.PageSize,
	})
	if err != nil {
		return OperatorOrganizationList{}, err
	}
	now := time.Now()
	items := make([]OperatorOrganization, 0, len(list.Items))
	for _, summary := range list.Items {
		items = append(items, o.operatorOrganization(summary, now))
	}
	return OperatorOrganizationList{Items: items, Total: list.Total}, nil
}

// GetOrganization 返回工作区摘要与状态。
func (o *operatorOperations) GetOrganization(ctx context.Context, meta OperatorRequestMeta, _ OperatorIdentity, organizationID string) (OperatorOrganization, error) {
	summary, err := o.organizationSummaries.Get(ctx, organizationID)
	if errors.Is(err, organizationaction.ErrNotFound) {
		return OperatorOrganization{}, newOperatorError(meta, http.StatusNotFound, OperatorErrorCodeOrganizationNotFound, cervii18n.ErrorOrganizationNotFound)
	}
	if err != nil {
		return OperatorOrganization{}, err
	}
	return o.operatorOrganization(summary, time.Now()), nil
}

// operatorOrganization 把工作区摘要转换为运营契约，按 now 计算权益有效性。
func (o *operatorOperations) operatorOrganization(summary organizationaction.Summary, now time.Time) OperatorOrganization {
	organization := OperatorOrganization{
		ID:              summary.ID,
		Name:            summary.Name,
		Slug:            summary.Slug,
		PublicURL:       WorkspaceURL(o.deployment.PublicURL, summary.Slug),
		LifecycleStatus: OrganizationLifecycleStatus(summary.LifecycleStatus),
		CreatedAt:       summary.CreatedAt,
	}
	if entitlement := summary.Entitlement; entitlement != nil {
		organization.Entitlement = &OperatorEntitlement{
			Revision:      entitlement.Revision,
			PlanCode:      entitlement.PlanCode,
			ServiceEndsAt: entitlement.ServiceEndsAt,
			Valid:         entitlement.ServiceEndsAt == nil || now.Before(*entitlement.ServiceEndsAt),
			AppliedAt:     entitlement.AppliedAt,
		}
	}
	return organization
}
