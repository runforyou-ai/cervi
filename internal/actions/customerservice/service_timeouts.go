//go:build server

package customerservice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const (
	ValidationTimeoutMinutesInvalid ValidationCode = "SERVICE_TIMEOUT_MINUTES_INVALID"
	ValidationReclaimNotAfterRemind ValidationCode = "SERVICE_TIMEOUT_RECLAIM_NOT_AFTER_REMINDER"
)

// LoadServiceTimeouts 读取企业客服超时时长，企业未设置时返回默认值。
func LoadServiceTimeouts(ctx context.Context, db bun.IDB, organizationID string) (domain.ServiceTimeouts, error) {
	setting := &servermodels.CustomerServiceSetting{}
	err := db.NewSelect().Model(setting).
		Column("response_reminder_minutes", "response_reclaim_minutes", "queue_reminder_minutes").
		Where("css.organization_id = ?", organizationID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DefaultServiceTimeouts(), nil
	}
	if err != nil {
		return domain.ServiceTimeouts{}, fmt.Errorf("load service timeouts: %w", err)
	}
	return domain.ServiceTimeouts{
		ResponseReminderMinutes: setting.ResponseReminderMinutes,
		ResponseReclaimMinutes:  setting.ResponseReclaimMinutes,
		QueueReminderMinutes:    setting.QueueReminderMinutes,
	}, nil
}

// GetServiceTimeoutsQuery 读取当前企业的客服超时时长。
type GetServiceTimeoutsQuery struct {
	db *bun.DB
}

// NewGetServiceTimeoutsQuery 创建客服超时时长读取查询。
func NewGetServiceTimeoutsQuery(db *bun.DB) *GetServiceTimeoutsQuery {
	return &GetServiceTimeoutsQuery{db: db}
}

// Execute 返回当前企业的客服超时时长。
func (q *GetServiceTimeoutsQuery) Execute(ctx context.Context, identity *servermodels.Identity) (domain.ServiceTimeouts, error) {
	return LoadServiceTimeouts(ctx, q.db, identity.Organization.ID)
}

// UpdateServiceTimeoutsAction 修改当前企业的客服超时时长。
type UpdateServiceTimeoutsAction struct {
	db *bun.DB
}

// NewUpdateServiceTimeoutsAction 创建客服超时时长修改操作。
func NewUpdateServiceTimeoutsAction(db *bun.DB) *UpdateServiceTimeoutsAction {
	return &UpdateServiceTimeoutsAction{db: db}
}

// Execute 校验并保存客服超时时长：各时长至少 1 分钟，回收时长大于提醒时长；企业尚无设置行时工作时间按默认值写入。
func (a *UpdateServiceTimeoutsAction) Execute(ctx context.Context, identity *servermodels.Identity, input domain.ServiceTimeouts) (domain.ServiceTimeouts, error) {
	fields := make(map[string]ValidationCode)
	if input.ResponseReminderMinutes < 1 {
		fields["responseReminderMinutes"] = ValidationTimeoutMinutesInvalid
	}
	if input.ResponseReclaimMinutes < 1 {
		fields["responseReclaimMinutes"] = ValidationTimeoutMinutesInvalid
	} else if input.ResponseReclaimMinutes <= input.ResponseReminderMinutes {
		fields["responseReclaimMinutes"] = ValidationReclaimNotAfterRemind
	}
	if input.QueueReminderMinutes < 1 {
		fields["queueReminderMinutes"] = ValidationTimeoutMinutesInvalid
	}
	if len(fields) > 0 {
		return domain.ServiceTimeouts{}, &ValidationError{Fields: fields}
	}
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		hours := domain.DefaultBusinessHours()
		setting := &servermodels.CustomerServiceSetting{
			OrganizationID: identity.Organization.ID, BusinessHoursEnabled: hours.Enabled, BusinessHoursTimeZone: hours.TimeZone,
			BusinessHoursWeekly: hours.Weekly[:], BusinessHoursOverrides: hours.Overrides,
			ResponseReminderMinutes: input.ResponseReminderMinutes, ResponseReclaimMinutes: input.ResponseReclaimMinutes,
			QueueReminderMinutes: input.QueueReminderMinutes,
		}
		if _, err := tx.NewInsert().Model(setting).
			Column("organization_id", "business_hours_enabled", "business_hours_time_zone", "business_hours_weekly", "business_hours_overrides",
				"response_reminder_minutes", "response_reclaim_minutes", "queue_reminder_minutes").
			On("CONFLICT (organization_id) DO UPDATE").
			Set("response_reminder_minutes = EXCLUDED.response_reminder_minutes").
			Set("response_reclaim_minutes = EXCLUDED.response_reclaim_minutes").
			Set("queue_reminder_minutes = EXCLUDED.queue_reminder_minutes").
			Set("updated_at = now()").
			Exec(ctx); err != nil {
			return fmt.Errorf("save service timeouts: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.ServiceTimeouts{}, err
	}
	return input, nil
}
