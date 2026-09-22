//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	organizationaction "github.com/runforyou-ai/cervi/internal/actions/organization"
	provisioningaction "github.com/runforyou-ai/cervi/internal/actions/provisioning"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// operatorOrganizationPageSizeMax 是运营企业列表单页的最大条数。
const operatorOrganizationPageSizeMax = 100

// CheckDomainAvailability 返回域名前缀在查询时刻是否可用。
func (o *operatorOperations) CheckDomainAvailability(ctx context.Context, meta OperatorRequestMeta, _ OperatorIdentity, input OperatorDomainAvailabilityInput) (OperatorDomainAvailability, error) {
	availability, err := o.checkDomain.Execute(ctx, input.Prefix)
	if err != nil {
		return OperatorDomainAvailability{}, operatorProvisioningError(meta, err)
	}
	return OperatorDomainAvailability{
		DomainPrefix: availability.DomainPrefix,
		AccessHost:   availability.AccessHost,
		PublicURL:    managedPublicURL(availability.AccessHost),
		Available:    availability.Available,
	}, nil
}

// ProvisionOrganization 按开通标识幂等地创建企业、初始成员和初始权益。
func (o *operatorOperations) ProvisionOrganization(ctx context.Context, meta OperatorRequestMeta, _ OperatorIdentity, input OperatorProvisionInput) (OperatorProvisioning, error) {
	result, err := o.provisionOrganization.Execute(ctx, provisioningaction.Input{
		ProvisioningID:   input.ProvisioningID,
		OrganizationName: input.OrganizationName,
		DomainPrefix:     input.DomainPrefix,
		InitialUser: provisioningaction.InitialUserInput{
			Subject:     input.InitialUser.Subject,
			DisplayName: input.InitialUser.DisplayName,
			Email:       input.InitialUser.Email,
			Locale:      domain.Locale(input.InitialUser.Locale),
			TimeZone:    input.InitialUser.TimeZone,
		},
		Entitlement: provisioningaction.EntitlementInput{
			Revision:      input.Entitlement.Revision,
			PlanCode:      input.Entitlement.PlanCode,
			ServiceEndsAt: input.Entitlement.ServiceEndsAt,
		},
	})
	if err != nil {
		return OperatorProvisioning{}, operatorProvisioningError(meta, err)
	}
	slog.Info("运营开通企业完成", "request_id", meta.RequestID, "provisioning_id", result.ProvisioningID,
		"organization_id", result.OrganizationID, "access_host", result.AccessHost)
	return operatorProvisioning(result), nil
}

// GetProvisioning 返回开通标识对应企业的当前状态。
func (o *operatorOperations) GetProvisioning(ctx context.Context, meta OperatorRequestMeta, _ OperatorIdentity, provisioningID string) (OperatorProvisioning, error) {
	result, err := o.getProvisioning.Execute(ctx, provisioningID)
	if err != nil {
		return OperatorProvisioning{}, operatorProvisioningError(meta, err)
	}
	return operatorProvisioning(result), nil
}

// ListOrganizations 按条件分页返回企业摘要。
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
		items = append(items, operatorOrganization(summary, now))
	}
	return OperatorOrganizationList{Items: items, Total: list.Total}, nil
}

// GetOrganization 返回企业摘要与状态。
func (o *operatorOperations) GetOrganization(ctx context.Context, meta OperatorRequestMeta, _ OperatorIdentity, organizationID string) (OperatorOrganization, error) {
	summary, err := o.organizationSummaries.Get(ctx, organizationID)
	if errors.Is(err, organizationaction.ErrNotFound) {
		return OperatorOrganization{}, newOperatorError(meta, http.StatusNotFound, OperatorErrorCodeOrganizationNotFound, cervii18n.ErrorOrganizationNotFound)
	}
	if err != nil {
		return OperatorOrganization{}, err
	}
	return operatorOrganization(summary, time.Now()), nil
}

// operatorProvisioningError 把开通领域错误转成带稳定错误码的运营错误，未分类错误原样返回。
func operatorProvisioningError(meta OperatorRequestMeta, err error) error {
	if fieldErr, ok := errors.AsType[*provisioningaction.ValidationError](err); ok {
		fields := make(map[string]string, len(fieldErr.Fields))
		for field, code := range fieldErr.Fields {
			fields[field] = string(code)
		}
		return operatorFieldError(meta, fields)
	}
	switch {
	case errors.Is(err, provisioningaction.ErrDomainPrefixInvalid):
		return newOperatorError(meta, http.StatusBadRequest, OperatorErrorCodeInvalidDomainPrefix, cervii18n.ErrorDomainPrefixInvalid)
	case errors.Is(err, provisioningaction.ErrDomainTaken):
		return newOperatorError(meta, http.StatusConflict, OperatorErrorCodeDomainTaken, cervii18n.ErrorDomainTaken)
	case errors.Is(err, provisioningaction.ErrIdempotencyConflict):
		return newOperatorError(meta, http.StatusConflict, OperatorErrorCodeIdempotencyConflict, cervii18n.ErrorProvisioningConflict)
	case errors.Is(err, provisioningaction.ErrNotFound):
		return newOperatorError(meta, http.StatusNotFound, OperatorErrorCodeProvisioningNotFound, cervii18n.ErrorProvisioningNotFound)
	}
	return err
}

// operatorProvisioning 把开通结果转换为运营契约。
func operatorProvisioning(result provisioningaction.Result) OperatorProvisioning {
	return OperatorProvisioning{
		ProvisioningID:      result.ProvisioningID,
		OrganizationID:      result.OrganizationID,
		AccessHost:          result.AccessHost,
		PublicURL:           managedPublicURL(result.AccessHost),
		InitialUserID:       result.InitialUserID,
		LifecycleStatus:     OrganizationLifecycleStatus(result.LifecycleStatus),
		EntitlementRevision: result.EntitlementRevision,
	}
}

// operatorOrganization 把企业摘要转换为运营契约，按 now 计算权益有效性。
func operatorOrganization(summary organizationaction.Summary, now time.Time) OperatorOrganization {
	organization := OperatorOrganization{
		ID:              summary.ID,
		Name:            summary.Name,
		AccessHost:      summary.AccessHost,
		PublicURL:       managedPublicURL(summary.AccessHost),
		LifecycleStatus: OrganizationLifecycleStatus(summary.LifecycleStatus),
		ProvisioningID:  summary.ProvisioningID,
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

// managedPublicURL 返回托管企业的公开 HTTPS 地址。
func managedPublicURL(accessHost string) string {
	return "https://" + accessHost
}
