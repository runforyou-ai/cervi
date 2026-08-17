-- +goose Up
-- 创建企业渠道表，关联关系由 Action 维护。
CREATE TABLE channels (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     uuid NOT NULL,
    created_by_user_id  uuid NOT NULL,
    type                text NOT NULL,
    name                text NOT NULL,
    description         text,
    default_locale      text NOT NULL DEFAULT 'zh-CN',
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz
);

CREATE INDEX channels_organization_type_deleted_at_index
    ON channels (organization_id, type, deleted_at);

-- +goose Down
DROP TABLE channels;
