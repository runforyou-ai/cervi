-- +goose Up
ALTER TABLE conversations RENAME COLUMN last_group_message_sequence TO last_message_seq;
ALTER TABLE messages RENAME COLUMN group_message_sequence TO message_seq;
ALTER TABLE messages ALTER COLUMN message_seq SET NOT NULL;
DROP INDEX messages_group_message_sequence_unique;
CREATE UNIQUE INDEX messages_message_seq_unique ON messages (organization_id, conversation_id, message_seq);
ALTER TABLE conversations DROP COLUMN last_message_source_order;
ALTER TABLE service_sessions DROP COLUMN last_message_source_order;
ALTER TABLE conversation_user_states ADD COLUMN read_seq bigint NOT NULL DEFAULT 0;
COMMENT ON COLUMN conversations.last_message_seq IS '会话已提交分配的最大消息序号，不随摘要重算回退';
COMMENT ON COLUMN messages.message_seq IS '会话内的消息写入顺序';
COMMENT ON INDEX messages_message_seq_unique IS '会话内消息序号唯一约束';
COMMENT ON COLUMN conversation_user_states.read_seq IS '用户已阅读的会话消息序号';

-- +goose Down
ALTER TABLE conversations ADD COLUMN last_message_source_order bigint NOT NULL DEFAULT 0;
ALTER TABLE service_sessions ADD COLUMN last_message_source_order bigint NOT NULL DEFAULT 0;
COMMENT ON COLUMN conversations.last_message_source_order IS '最后消息的来源顺序';
COMMENT ON COLUMN service_sessions.last_message_source_order IS '最后消息的来源顺序';
ALTER TABLE conversation_user_states DROP COLUMN read_seq;
DROP INDEX messages_message_seq_unique;
ALTER TABLE messages ALTER COLUMN message_seq DROP NOT NULL;
ALTER TABLE messages RENAME COLUMN message_seq TO group_message_sequence;
ALTER TABLE conversations RENAME COLUMN last_message_seq TO last_group_message_sequence;
CREATE UNIQUE INDEX messages_group_message_sequence_unique ON messages (organization_id, conversation_id, group_message_sequence) WHERE group_message_sequence IS NOT NULL;
COMMENT ON COLUMN conversations.last_group_message_sequence IS '群聊已分配的最大消息序号';
COMMENT ON COLUMN messages.group_message_sequence IS '群聊内的消息写入顺序';
COMMENT ON INDEX messages_group_message_sequence_unique IS '群聊内消息序号唯一约束';
