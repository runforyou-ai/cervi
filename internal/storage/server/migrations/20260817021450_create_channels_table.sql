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
    new_conversation_target_type  text NOT NULL DEFAULT 'public_queue',
    new_conversation_target_id    uuid,
    fallback_target_type          text NOT NULL DEFAULT 'public_queue',
    fallback_target_id            uuid,
    enabled             boolean NOT NULL DEFAULT true,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE channels IS '企业消息渠道';
COMMENT ON COLUMN channels.id IS '渠道编号';
COMMENT ON COLUMN channels.organization_id IS '所属企业编号';
COMMENT ON COLUMN channels.created_by_user_id IS '创建人编号';
COMMENT ON COLUMN channels.type IS '渠道类型';
COMMENT ON COLUMN channels.name IS '渠道名称';
COMMENT ON COLUMN channels.description IS '渠道描述';
COMMENT ON COLUMN channels.default_locale IS '默认接待语言';
COMMENT ON COLUMN channels.new_conversation_target_type IS '新会话进入目标类型';
COMMENT ON COLUMN channels.new_conversation_target_id IS '新会话进入团队或成员编号';
COMMENT ON COLUMN channels.fallback_target_type IS '无法处理时的目标类型';
COMMENT ON COLUMN channels.fallback_target_id IS '无法处理时的团队或成员编号';
COMMENT ON COLUMN channels.enabled IS '是否启用';
COMMENT ON COLUMN channels.created_at IS '创建时间';
COMMENT ON COLUMN channels.updated_at IS '更新时间';

CREATE INDEX channels_organization_type_enabled_index
    ON channels (organization_id, type, enabled);

-- +goose Down
DROP TABLE channels;
