-- +goose Up
-- 创建企业对象存储设置表，关联关系由 Action 维护。
CREATE TABLE settings (
    organization_id  uuid NOT NULL,
    key              text NOT NULL,
    value            jsonb NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (organization_id, key)
);

-- +goose Down
DROP TABLE settings;
