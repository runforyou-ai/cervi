-- +goose Up
-- 创建会话消息表。
CREATE TABLE messages (
    id                     uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),
    organization_id        uuid NOT NULL,
    conversation_id        uuid NOT NULL,
    service_session_id     uuid,
    sender_participant_id  uuid,
    type                   text NOT NULL,
    body                   text NOT NULL DEFAULT '',
    reply_to_message_id    uuid,
    thread_root_message_id uuid,
    idempotency_key        text,
    originated_at          timestamptz NOT NULL,
    source_order           bigint NOT NULL DEFAULT 0,
    edited_at              timestamptz,
    deleted_at             timestamptz
);

CREATE UNIQUE INDEX messages_organization_idempotency_unique
    ON messages (organization_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

COMMENT ON TABLE messages IS '会话消息';
COMMENT ON COLUMN messages.id IS '消息编号';
COMMENT ON COLUMN messages.created_at IS '创建时间';
COMMENT ON COLUMN messages.updated_at IS '更新时间';
COMMENT ON COLUMN messages.organization_id IS '所属企业编号';
COMMENT ON COLUMN messages.conversation_id IS '所属会话编号';
COMMENT ON COLUMN messages.service_session_id IS '所属客服处理周期编号';
COMMENT ON COLUMN messages.sender_participant_id IS '发送参与者编号';
COMMENT ON COLUMN messages.type IS '消息类型：text、system';
COMMENT ON COLUMN messages.body IS '消息文本内容';
COMMENT ON COLUMN messages.reply_to_message_id IS '回复目标消息编号';
COMMENT ON COLUMN messages.thread_root_message_id IS '讨论串根消息编号';
COMMENT ON COLUMN messages.idempotency_key IS '消息写入幂等标识';
COMMENT ON COLUMN messages.originated_at IS '消息在来源端发生时间';
COMMENT ON COLUMN messages.source_order IS '同一来源时间内的平台消息顺序，站内消息为零';
COMMENT ON COLUMN messages.edited_at IS '最后编辑时间';
COMMENT ON COLUMN messages.deleted_at IS '删除时间';
COMMENT ON INDEX messages_organization_idempotency_unique
    IS '企业消息幂等标识唯一索引';

-- +goose Down
DROP TABLE messages;
