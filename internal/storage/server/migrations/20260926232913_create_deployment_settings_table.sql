-- +goose Up
-- 创建部署设置表。
CREATE TABLE deployment_settings (
    id                 smallint PRIMARY KEY DEFAULT 1,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    registration_open  boolean NOT NULL DEFAULT false
);

COMMENT ON TABLE deployment_settings IS '部署级设置，至多一行，缺失时按默认值处理';
COMMENT ON COLUMN deployment_settings.id IS '固定为 1 的设置行编号';
COMMENT ON COLUMN deployment_settings.created_at IS '创建时间';
COMMENT ON COLUMN deployment_settings.updated_at IS '更新时间';
COMMENT ON COLUMN deployment_settings.registration_open IS '是否开放账号注册';

-- +goose Down
DROP TABLE deployment_settings;
