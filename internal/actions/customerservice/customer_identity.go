//go:build server

package customerservice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common/customeridentity"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// LoadCustomerIdentitySecret 读取企业客户身份密钥，未生成时返回空串。
func LoadCustomerIdentitySecret(ctx context.Context, db bun.IDB, organizationID string) (string, error) {
	setting := &servermodels.CustomerServiceSetting{}
	err := db.NewSelect().Model(setting).
		Column("customer_identity_secret").
		Where("css.organization_id = ?", organizationID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && setting.CustomerIdentitySecret == nil) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("load customer identity secret: %w", err)
	}
	return *setting.CustomerIdentitySecret, nil
}

// GetCustomerIdentitySecretQuery 读取当前企业的客户身份密钥。
type GetCustomerIdentitySecretQuery struct {
	db *bun.DB
}

// NewGetCustomerIdentitySecretQuery 创建客户身份密钥读取查询。
func NewGetCustomerIdentitySecretQuery(db *bun.DB) *GetCustomerIdentitySecretQuery {
	return &GetCustomerIdentitySecretQuery{db: db}
}

// Execute 返回当前企业的客户身份密钥，未生成时返回空串。
func (q *GetCustomerIdentitySecretQuery) Execute(ctx context.Context, identity *servermodels.Identity) (string, error) {
	return LoadCustomerIdentitySecret(ctx, q.db, identity.Organization.ID)
}

// RegenerateCustomerIdentitySecretAction 生成或重新生成当前企业的客户身份密钥。
type RegenerateCustomerIdentitySecretAction struct {
	db *bun.DB
}

// NewRegenerateCustomerIdentitySecretAction 创建客户身份密钥生成操作。
func NewRegenerateCustomerIdentitySecretAction(db *bun.DB) *RegenerateCustomerIdentitySecretAction {
	return &RegenerateCustomerIdentitySecretAction{db: db}
}

// Execute 保存新密钥并在提交后结束该企业全部以签名身份建立的访客事件流；企业尚无设置行时其余设置按默认值写入。
func (a *RegenerateCustomerIdentitySecretAction) Execute(ctx context.Context, identity *servermodels.Identity) (string, error) {
	secret, err := customeridentity.GenerateSecret()
	if err != nil {
		return "", err
	}
	err = realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		hours := domain.DefaultBusinessHours()
		timeouts := domain.DefaultServiceTimeouts()
		setting := &servermodels.CustomerServiceSetting{
			OrganizationID: identity.Organization.ID, BusinessHoursEnabled: hours.Enabled, BusinessHoursTimeZone: hours.TimeZone,
			BusinessHoursWeekly: hours.Weekly[:], BusinessHoursOverrides: hours.Overrides,
			ResponseReminderMinutes: timeouts.ResponseReminderMinutes, ResponseReclaimMinutes: timeouts.ResponseReclaimMinutes,
			QueueReminderMinutes: timeouts.QueueReminderMinutes, AIFollowUpMinutes: timeouts.AIFollowUpMinutes, AICloseMinutes: timeouts.AICloseMinutes,
			CustomerIdentitySecret: &secret,
		}
		if _, err := tx.NewInsert().Model(setting).
			Column("organization_id", "business_hours_enabled", "business_hours_time_zone", "business_hours_weekly", "business_hours_overrides",
				"response_reminder_minutes", "response_reclaim_minutes", "queue_reminder_minutes", "ai_follow_up_minutes", "ai_close_minutes",
				"customer_identity_secret").
			On("CONFLICT (organization_id) DO UPDATE").
			Set("customer_identity_secret = EXCLUDED.customer_identity_secret").
			Set("updated_at = now()").
			Exec(ctx); err != nil {
			return fmt.Errorf("save customer identity secret: %w", err)
		}
		realtime.Notify(ctx, realtime.CustomerIdentityRevoked(identity.Organization.ID))
		return nil
	})
	if err != nil {
		return "", err
	}
	return secret, nil
}
