-- +goose Up
-- 创建企业渠道表。
CREATE TABLE channels (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
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

COMMENT ON TABLE channels IS '企业消息渠道';
COMMENT ON COLUMN channels.id IS '渠道编号';
COMMENT ON COLUMN channels.organization_id IS '所属企业编号';
COMMENT ON COLUMN channels.created_by_user_id IS '创建人编号';
COMMENT ON COLUMN channels.type IS '渠道类型';
COMMENT ON COLUMN channels.name IS '渠道名称';
COMMENT ON COLUMN channels.description IS '渠道描述';
COMMENT ON COLUMN channels.default_locale IS '默认语言';
COMMENT ON COLUMN channels.created_at IS '创建时间';
COMMENT ON COLUMN channels.updated_at IS '更新时间';
COMMENT ON COLUMN channels.deleted_at IS '移入回收站时间';

CREATE INDEX channels_organization_type_deleted_at_index
    ON channels (organization_id, type, deleted_at);

-- +goose Down
DROP TABLE channels;
