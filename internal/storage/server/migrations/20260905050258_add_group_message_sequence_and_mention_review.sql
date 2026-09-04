-- +goose Up
ALTER TABLE conversations ADD COLUMN message_sequence bigint NOT NULL DEFAULT 0;
ALTER TABLE messages ADD COLUMN conversation_sequence bigint;
ALTER TABLE conversation_user_states ADD COLUMN last_reviewed_mention_message_id uuid;

CREATE UNIQUE INDEX messages_conversation_sequence_unique
    ON messages (organization_id, conversation_id, conversation_sequence)
    WHERE conversation_sequence IS NOT NULL;

COMMENT ON COLUMN conversations.message_sequence IS '群聊已分配的最大消息序号';
COMMENT ON COLUMN messages.conversation_sequence IS '群聊内的消息写入顺序';
COMMENT ON COLUMN conversation_user_states.last_reviewed_mention_message_id IS '已连续查看的提及或本轮入群基线消息编号';
COMMENT ON INDEX messages_conversation_sequence_unique IS '群聊内消息序号唯一约束';

-- +goose Down
DROP INDEX messages_conversation_sequence_unique;
ALTER TABLE conversation_user_states DROP COLUMN last_reviewed_mention_message_id;
ALTER TABLE messages DROP COLUMN conversation_sequence;
ALTER TABLE conversations DROP COLUMN message_sequence;
