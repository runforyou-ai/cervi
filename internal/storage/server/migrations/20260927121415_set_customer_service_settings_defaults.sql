-- +goose Up
-- 客服设置行随工作区创建，工作时间列给出默认值。
ALTER TABLE customer_service_settings
    ALTER COLUMN business_hours_enabled SET DEFAULT false,
    ALTER COLUMN business_hours_time_zone SET DEFAULT 'Asia/Shanghai',
    ALTER COLUMN business_hours_weekly SET DEFAULT '[[{"start":"09:00","end":"18:00"}],[{"start":"09:00","end":"18:00"}],[{"start":"09:00","end":"18:00"}],[{"start":"09:00","end":"18:00"}],[{"start":"09:00","end":"18:00"}],[],[]]'::jsonb,
    ALTER COLUMN business_hours_overrides SET DEFAULT '[]'::jsonb;

COMMENT ON TABLE customer_service_settings IS '工作区客服设置，每个工作区一行，创建工作区时写入';

-- +goose Down
ALTER TABLE customer_service_settings
    ALTER COLUMN business_hours_enabled DROP DEFAULT,
    ALTER COLUMN business_hours_time_zone DROP DEFAULT,
    ALTER COLUMN business_hours_weekly DROP DEFAULT,
    ALTER COLUMN business_hours_overrides DROP DEFAULT;

COMMENT ON TABLE customer_service_settings IS '工作区客服设置，每个工作区至多一行，缺失时按默认值处理';
