-- +goose Up
-- 创建企业设置表。
CREATE TABLE settings (
    organization_id  uuid NOT NULL,
    key              text NOT NULL,
    PRIMARY KEY (organization_id, key),

    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    value            jsonb NOT NULL
);

COMMENT ON TABLE settings IS '企业级键值设置';
COMMENT ON COLUMN settings.organization_id IS '所属企业编号';
COMMENT ON COLUMN settings.key IS '设置键';
COMMENT ON COLUMN settings.created_at IS '创建时间';
COMMENT ON COLUMN settings.updated_at IS '更新时间';
COMMENT ON COLUMN settings.value IS '设置内容';

-- +goose Down
DROP TABLE settings;
