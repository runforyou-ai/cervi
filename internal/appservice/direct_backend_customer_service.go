//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	customerserviceaction "github.com/runforyou-ai/cervi/internal/actions/customerservice"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// customerServiceOps 持有企业客服设置的业务实现依赖。
type customerServiceOps struct {
	getBusinessHours    *customerserviceaction.GetBusinessHoursQuery
	updateBusinessHours *customerserviceaction.UpdateBusinessHoursAction
}

// newCustomerServiceOps 创建企业客服设置的业务实现依赖。
func newCustomerServiceOps(db *bun.DB) customerServiceOps {
	return customerServiceOps{
		getBusinessHours:    customerserviceaction.NewGetBusinessHoursQuery(db),
		updateBusinessHours: customerserviceaction.NewUpdateBusinessHoursAction(db),
	}
}

// GetBusinessHours 读取当前企业的客服工作时间。
func (o *directOperations) GetBusinessHours(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (BusinessHours, error) {
	hours, err := o.getBusinessHours.Execute(ctx, identity)
	if err != nil {
		if ctx.Err() != nil {
			return BusinessHours{}, ctx.Err()
		}
		slog.Warn("读取客服工作时间失败", "organization_id", identity.Organization.ID, "error", err)
		return BusinessHours{}, FailedError(meta, cervii18n.ErrorBusinessHoursLoadFailed)
	}
	return businessHoursFromDomain(hours), nil
}

// UpdateBusinessHours 修改当前企业的客服工作时间。
func (o *directOperations) UpdateBusinessHours(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input BusinessHours) (BusinessHours, error) {
	// 每周时段固定 7 项，按周一到周日转换为领域值。
	if len(input.Weekly) != 7 {
		return BusinessHours{}, InvalidError(meta, cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{"weekly": cervii18n.FieldBusinessHoursWeeklyInvalid})
	}
	hours := domain.BusinessHours{Enabled: input.Enabled, TimeZone: input.TimeZone, Overrides: make([]domain.BusinessHoursOverride, 0, len(input.Overrides))}
	for day, periods := range input.Weekly {
		hours.Weekly[day] = businessHoursPeriodsToDomain(periods)
	}
	for _, override := range input.Overrides {
		hours.Overrides = append(hours.Overrides, domain.BusinessHoursOverride{Date: override.Date, Periods: businessHoursPeriodsToDomain(override.Periods)})
	}
	saved, err := o.updateBusinessHours.Execute(ctx, identity, hours)
	if err != nil {
		if ctx.Err() != nil {
			return BusinessHours{}, ctx.Err()
		}
		if validationError, ok := errors.AsType[*common.FieldError](err); ok {
			// 把客服工作时间校验错误码映射为本地化文案键。
			keys := map[common.FieldCode]cervii18n.Key{
				customerserviceaction.ValidationTimeZoneInvalid: cervii18n.FieldTimeZoneInvalid,
				customerserviceaction.ValidationWeeklyInvalid:   cervii18n.FieldBusinessHoursWeeklyInvalid,
				customerserviceaction.ValidationOverrideInvalid: cervii18n.FieldBusinessHoursOverrideInvalid,
			}
			return BusinessHours{}, InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, keys))
		}
		if errors.Is(err, common.ErrIdentityInvalid) {
			return BusinessHours{}, SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
		}
		slog.Warn("修改客服工作时间失败", "organization_id", identity.Organization.ID, "error", err)
		return BusinessHours{}, FailedError(meta, cervii18n.ErrorBusinessHoursUpdateFailed)
	}
	slog.Info("客服工作时间已更新", "organization_id", identity.Organization.ID, "enabled", saved.Enabled, "time_zone", saved.TimeZone)
	return businessHoursFromDomain(saved), nil
}

// businessHoursFromDomain 把领域工作时间转换为传输结构。
func businessHoursFromDomain(hours domain.BusinessHours) BusinessHours {
	output := BusinessHours{Enabled: hours.Enabled, TimeZone: hours.TimeZone, Weekly: make([][]BusinessHoursPeriod, 0, len(hours.Weekly)), Overrides: make([]BusinessHoursOverride, 0, len(hours.Overrides))}
	for _, periods := range hours.Weekly {
		output.Weekly = append(output.Weekly, businessHoursPeriodsFromDomain(periods))
	}
	for _, override := range hours.Overrides {
		output.Overrides = append(output.Overrides, BusinessHoursOverride{Date: override.Date, Periods: businessHoursPeriodsFromDomain(override.Periods)})
	}
	return output
}

// businessHoursPeriodsFromDomain 把领域工作时段转换为传输结构。
func businessHoursPeriodsFromDomain(periods []domain.BusinessHoursPeriod) []BusinessHoursPeriod {
	output := make([]BusinessHoursPeriod, 0, len(periods))
	for _, period := range periods {
		output = append(output, BusinessHoursPeriod{Start: period.Start, End: period.End})
	}
	return output
}

// businessHoursPeriodsToDomain 把传输结构的工作时段转换为领域值。
func businessHoursPeriodsToDomain(periods []BusinessHoursPeriod) []domain.BusinessHoursPeriod {
	output := make([]domain.BusinessHoursPeriod, 0, len(periods))
	for _, period := range periods {
		output = append(output, domain.BusinessHoursPeriod{Start: period.Start, End: period.End})
	}
	return output
}
