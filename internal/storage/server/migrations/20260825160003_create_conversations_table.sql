-- +goose Up
-- 创建聊天会话表。
CREATE TABLE conversations (
    id                     uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),
    organization_id        uuid NOT NULL,
    type                   text NOT NULL,
    status                 text NOT NULL DEFAULT 'active',
    title                  text,
    created_by_subject_id  uuid,
    last_message_id        uuid,
    last_message_at        timestamptz
);

COMMENT ON TABLE conversations IS '聊天会话';
COMMENT ON COLUMN conversations.id IS '会话编号';
COMMENT ON COLUMN conversations.created_at IS '创建时间';
COMMENT ON COLUMN conversations.updated_at IS '更新时间';
COMMENT ON COLUMN conversations.organization_id IS '所属企业编号';
COMMENT ON COLUMN conversations.type IS '会话类型：direct、group、customer';
COMMENT ON COLUMN conversations.status IS '会话生命周期状态：active、archived';
COMMENT ON COLUMN conversations.title IS '会话标题';
COMMENT ON COLUMN conversations.created_by_subject_id IS '创建聊天主体编号';
COMMENT ON COLUMN conversations.last_message_id IS '会话最后消息编号';
COMMENT ON COLUMN conversations.last_message_at IS '会话最后消息发生时间';

CREATE INDEX conversations_org_type_status_last_message_index
    ON conversations (
        organization_id,
        type,
        status,
        last_message_at DESC NULLS LAST,
        id DESC
    );

COMMENT ON INDEX conversations_org_type_status_last_message_index
    IS '企业会话按类型、状态和最后消息排序索引';

-- +goose Down
DROP TABLE conversations;
