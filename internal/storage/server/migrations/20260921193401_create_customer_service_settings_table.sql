-- +goose Up
-- 创建企业客服设置表。
CREATE TABLE customer_service_settings (
    organization_id           uuid PRIMARY KEY,

    created_at                timestamptz NOT NULL DEFAULT now(),
    updated_at                timestamptz NOT NULL DEFAULT now(),
    business_hours_enabled    boolean NOT NULL,
    business_hours_time_zone  text NOT NULL,
    business_hours_weekly     jsonb NOT NULL,
    business_hours_overrides  jsonb NOT NULL
);

COMMENT ON TABLE customer_service_settings IS '企业客服设置，每个企业至多一行，缺失时按默认值处理';
COMMENT ON COLUMN customer_service_settings.organization_id IS '所属企业编号';
COMMENT ON COLUMN customer_service_settings.created_at IS '创建时间';
COMMENT ON COLUMN customer_service_settings.updated_at IS '更新时间';
COMMENT ON COLUMN customer_service_settings.business_hours_enabled IS '是否启用工作时间，未启用时始终按工作时间处理';
COMMENT ON COLUMN customer_service_settings.business_hours_time_zone IS '工作时间使用的 IANA 时区';
COMMENT ON COLUMN customer_service_settings.business_hours_weekly IS '周一至周日各自的工作时段列表，每段为 HH:mm 起止';
COMMENT ON COLUMN customer_service_settings.business_hours_overrides IS '按日期覆盖的工作时段，时段为空表示当天休息';

-- +goose Down
DROP TABLE customer_service_settings;
