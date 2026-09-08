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
    deleted_at             timestamptz,
    system_event_type      text,
    system_event_payload   jsonb,
    mention_all            boolean NOT NULL DEFAULT false,
    message_seq            bigint NOT NULL,
    client_message_id      uuid
);

CREATE UNIQUE INDEX messages_organization_idempotency_unique
    ON messages (organization_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE UNIQUE INDEX messages_message_seq_unique
    ON messages (organization_id, conversation_id, message_seq);

COMMENT ON TABLE messages IS '会话消息';
COMMENT ON COLUMN messages.id IS '消息编号';
COMMENT ON COLUMN messages.created_at IS '创建时间';
COMMENT ON COLUMN messages.updated_at IS '更新时间';
COMMENT ON COLUMN messages.organization_id IS '所属企业编号';
COMMENT ON COLUMN messages.conversation_id IS '所属会话编号';
COMMENT ON COLUMN messages.service_session_id IS '所属客服处理周期编号';
COMMENT ON COLUMN messages.sender_participant_id IS '发送参与者编号';
COMMENT ON COLUMN messages.type IS '消息类型：text 文本、system 系统事件、agent_error AI 运行失败、attachment 附件';
COMMENT ON COLUMN messages.body IS '消息文本内容';
COMMENT ON COLUMN messages.reply_to_message_id IS '回复目标消息编号';
COMMENT ON COLUMN messages.thread_root_message_id IS '讨论串根消息编号';
COMMENT ON COLUMN messages.idempotency_key IS '消息写入幂等标识';
COMMENT ON COLUMN messages.originated_at IS '消息在来源端发生时间';
COMMENT ON COLUMN messages.source_order IS '同一来源时间内的平台消息顺序，站内消息为零';
COMMENT ON COLUMN messages.edited_at IS '最后编辑时间';
COMMENT ON COLUMN messages.deleted_at IS '删除时间';
COMMENT ON COLUMN messages.system_event_type IS '系统事件类型';
COMMENT ON COLUMN messages.system_event_payload IS '系统事件的类型化审计载荷';
COMMENT ON COLUMN messages.mention_all IS '是否提醒群聊中的所有成员';
COMMENT ON COLUMN messages.message_seq IS '会话内的消息写入顺序';
COMMENT ON COLUMN messages.client_message_id IS '发送方客户端消息编号';
COMMENT ON INDEX messages_organization_idempotency_unique
    IS '企业消息幂等标识唯一索引';
COMMENT ON INDEX messages_message_seq_unique IS '会话内消息序号唯一约束';

-- +goose Down
DROP TABLE messages;
