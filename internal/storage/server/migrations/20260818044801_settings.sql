-- +goose Up
-- 创建企业设置表，关联关系由 Action 维护。
CREATE TABLE settings (
    organization_id  uuid NOT NULL,
    key              text NOT NULL,
    value            jsonb NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (organization_id, key)
);

COMMENT ON TABLE settings IS '企业级键值设置';
COMMENT ON COLUMN settings.organization_id IS '所属企业编号';
COMMENT ON COLUMN settings.key IS '设置键';
COMMENT ON COLUMN settings.value IS '设置内容';
COMMENT ON COLUMN settings.created_at IS '创建时间';
COMMENT ON COLUMN settings.updated_at IS '更新时间';

-- +goose Down
DROP TABLE settings;
