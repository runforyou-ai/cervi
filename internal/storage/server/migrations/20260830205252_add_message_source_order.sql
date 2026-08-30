-- +goose Up
-- 为消息及其摘要增加来源内顺序。
ALTER TABLE messages
    ADD COLUMN source_order bigint NOT NULL DEFAULT 0;

COMMENT ON COLUMN messages.source_order IS '同一来源时间内的平台消息顺序，站内消息为零';

ALTER TABLE conversations
    ADD COLUMN last_message_source_order bigint NOT NULL DEFAULT 0;

COMMENT ON COLUMN conversations.last_message_source_order IS '最后消息的来源内顺序';

ALTER TABLE service_sessions
    ADD COLUMN last_message_source_order bigint NOT NULL DEFAULT 0;

COMMENT ON COLUMN service_sessions.last_message_source_order IS '最后消息的来源内顺序';

DROP INDEX messages_organization_conversation_originated_index;

CREATE INDEX messages_organization_conversation_originated_index
    ON messages (
        organization_id,
        conversation_id,
        originated_at DESC,
        source_order DESC,
        id DESC
    );

COMMENT ON INDEX messages_organization_conversation_originated_index
    IS '企业会话消息来源稳定顺序索引';

-- +goose Down
DROP INDEX messages_organization_conversation_originated_index;

CREATE INDEX messages_organization_conversation_originated_index
    ON messages (
        organization_id,
        conversation_id,
        originated_at DESC,
        id DESC
    );

COMMENT ON INDEX messages_organization_conversation_originated_index
    IS '企业会话消息发生时间索引';

ALTER TABLE service_sessions
    DROP COLUMN last_message_source_order;

ALTER TABLE conversations
    DROP COLUMN last_message_source_order;

ALTER TABLE messages
    DROP COLUMN source_order;
