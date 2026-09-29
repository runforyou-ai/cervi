-- +goose Up
-- 为用户会话状态增加归档时间。
ALTER TABLE conversation_user_states ADD COLUMN archived_at timestamptz;

COMMENT ON COLUMN conversation_user_states.archived_at IS '个人归档时间，会话出现新的对话消息时清空，未归档时为空';

-- +goose Down
ALTER TABLE conversation_user_states DROP COLUMN archived_at;
