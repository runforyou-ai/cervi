//go:build server

// Package customerservice 实现企业客服设置的读取与修改。
package customerservice

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	commontimezone "github.com/runforyou-ai/cervi/internal/common/timezone"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ValidationCode 标识企业客服设置的校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationTimeZoneInvalid ValidationCode = "BUSINESS_HOURS_TIME_ZONE_INVALID"
	ValidationWeeklyInvalid   ValidationCode = "BUSINESS_HOURS_WEEKLY_INVALID"
	ValidationOverrideInvalid ValidationCode = "BUSINESS_HOURS_OVERRIDE_INVALID"
)

// ValidationError 表示企业客服设置校验失败。
type ValidationError = common.FieldError

// LoadBusinessHours 读取企业客服工作时间，企业未设置时返回默认值。
func LoadBusinessHours(ctx context.Context, db bun.IDB, organizationID string) (domain.BusinessHours, error) {
	setting := &servermodels.CustomerServiceSetting{}
	err := db.NewSelect().Model(setting).
		Column("business_hours_enabled", "business_hours_time_zone", "business_hours_weekly", "business_hours_overrides").
		Where("css.organization_id = ?", organizationID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DefaultBusinessHours(), nil
	}
	if err != nil {
		return domain.BusinessHours{}, fmt.Errorf("load business hours: %w", err)
	}
	hours := domain.BusinessHours{Enabled: setting.BusinessHoursEnabled, TimeZone: setting.BusinessHoursTimeZone, Overrides: setting.BusinessHoursOverrides}
	for day := range hours.Weekly {
		hours.Weekly[day] = setting.BusinessHoursWeekly[day]
	}
	return hours, nil
}

// GetBusinessHoursQuery 读取当前企业的客服工作时间。
type GetBusinessHoursQuery struct {
	db *bun.DB
}

// NewGetBusinessHoursQuery 创建客服工作时间读取查询。
func NewGetBusinessHoursQuery(db *bun.DB) *GetBusinessHoursQuery {
	return &GetBusinessHoursQuery{db: db}
}

// Execute 返回当前企业的客服工作时间。
func (q *GetBusinessHoursQuery) Execute(ctx context.Context, identity *servermodels.Identity) (domain.BusinessHours, error) {
	return LoadBusinessHours(ctx, q.db, identity.Organization.ID)
}

// UpdateBusinessHoursAction 修改当前企业的客服工作时间。
type UpdateBusinessHoursAction struct {
	db *bun.DB
}

// NewUpdateBusinessHoursAction 创建客服工作时间修改操作。
func NewUpdateBusinessHoursAction(db *bun.DB) *UpdateBusinessHoursAction {
	return &UpdateBusinessHoursAction{db: db}
}

// Execute 校验并保存客服工作时间：时段按开始时间排序，日期覆盖按日期排序。
func (a *UpdateBusinessHoursAction) Execute(ctx context.Context, identity *servermodels.Identity, input domain.BusinessHours) (domain.BusinessHours, error) {
	fields := make(map[string]ValidationCode)
	if !commontimezone.Valid(input.TimeZone) {
		fields["timeZone"] = ValidationTimeZoneInvalid
	}
	for day := range input.Weekly {
		if !domain.BusinessHoursPeriodsValid(input.Weekly[day]) {
			fields["weekly"] = ValidationWeeklyInvalid
		}
		input.Weekly[day] = sortedPeriods(input.Weekly[day])
	}
	// 日期覆盖的日期必须合法且不重复，时段规则与每周时段相同。
	dates := make(map[string]struct{}, len(input.Overrides))
	overrides := make([]domain.BusinessHoursOverride, 0, len(input.Overrides))
	for _, override := range input.Overrides {
		_, dateErr := time.Parse(domain.BusinessHoursDateLayout, override.Date)
		_, duplicate := dates[override.Date]
		if dateErr != nil || duplicate || !domain.BusinessHoursPeriodsValid(override.Periods) {
			fields["overrides"] = ValidationOverrideInvalid
		}
		dates[override.Date] = struct{}{}
		overrides = append(overrides, domain.BusinessHoursOverride{Date: override.Date, Periods: sortedPeriods(override.Periods)})
	}
	if len(fields) > 0 {
		return domain.BusinessHours{}, &ValidationError{Fields: fields}
	}
	slices.SortFunc(overrides, func(left, right domain.BusinessHoursOverride) int { return cmp.Compare(left.Date, right.Date) })
	input.Overrides = overrides
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		setting := &servermodels.CustomerServiceSetting{
			OrganizationID: identity.Organization.ID, BusinessHoursEnabled: input.Enabled, BusinessHoursTimeZone: input.TimeZone,
			BusinessHoursWeekly: input.Weekly[:], BusinessHoursOverrides: input.Overrides,
		}
		if _, err := tx.NewInsert().Model(setting).
			Column("organization_id", "business_hours_enabled", "business_hours_time_zone", "business_hours_weekly", "business_hours_overrides").
			On("CONFLICT (organization_id) DO UPDATE").
			Set("business_hours_enabled = EXCLUDED.business_hours_enabled").
			Set("business_hours_time_zone = EXCLUDED.business_hours_time_zone").
			Set("business_hours_weekly = EXCLUDED.business_hours_weekly").
			Set("business_hours_overrides = EXCLUDED.business_hours_overrides").
			Set("updated_at = now()").
			Exec(ctx); err != nil {
			return fmt.Errorf("save business hours: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.BusinessHours{}, err
	}
	return input, nil
}

// sortedPeriods 返回按开始时间排序的时段副本，nil 输出为空列表。
func sortedPeriods(periods []domain.BusinessHoursPeriod) []domain.BusinessHoursPeriod {
	sorted := append([]domain.BusinessHoursPeriod{}, periods...)
	slices.SortFunc(sorted, func(left, right domain.BusinessHoursPeriod) int { return cmp.Compare(left.Start, right.Start) })
	return sorted
}
