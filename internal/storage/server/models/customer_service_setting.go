//go:build server

package models

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/uptrace/bun"
)

// CustomerServiceSetting 表示企业客服设置，每个企业至多一行。
type CustomerServiceSetting struct {
	bun.BaseModel `bun:"table:customer_service_settings,alias:css"`

	OrganizationID         string                         `bun:"organization_id,pk"`
	BusinessHoursEnabled   bool                           `bun:"business_hours_enabled"`
	BusinessHoursTimeZone  string                         `bun:"business_hours_time_zone"`
	BusinessHoursWeekly    [][]domain.BusinessHoursPeriod `bun:"business_hours_weekly,type:jsonb"`
	BusinessHoursOverrides []domain.BusinessHoursOverride `bun:"business_hours_overrides,type:jsonb"`
	CreatedAt              time.Time                      `bun:"created_at"`
	UpdatedAt              time.Time                      `bun:"updated_at"`
}
