-- +goose Up
ALTER TABLE agent_conversations
    ADD COLUMN memory_extracted_seq bigint NOT NULL DEFAULT 0;

COMMENT ON COLUMN agent_conversations.memory_extracted_seq IS '助理记忆已提取到的会话消息序号';

-- +goose Down
ALTER TABLE agent_conversations
    DROP COLUMN memory_extracted_seq;
