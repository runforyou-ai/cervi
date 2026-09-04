-- +goose Up
ALTER TABLE messages
    ADD COLUMN mention_all boolean NOT NULL DEFAULT false;

ALTER TABLE conversation_user_states
    ALTER COLUMN last_read_message_id DROP NOT NULL,
    ALTER COLUMN last_read_at DROP NOT NULL,
    ALTER COLUMN last_read_at DROP DEFAULT,
    ADD COLUMN muted boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN messages.mention_all IS '是否提醒群聊中的所有成员';
COMMENT ON COLUMN conversation_user_states.last_read_message_id IS '最后已读消息编号，尚未产生已读水位时为空';
COMMENT ON COLUMN conversation_user_states.last_read_at IS '最后标记已读时间，尚未产生已读水位时为空';
COMMENT ON COLUMN conversation_user_states.muted IS '是否降低当前用户在会话中的消息提醒';

-- +goose Down
ALTER TABLE conversation_user_states
    DROP COLUMN muted;

DELETE FROM conversation_user_states
    WHERE last_read_message_id IS NULL OR last_read_at IS NULL;

ALTER TABLE conversation_user_states
    ALTER COLUMN last_read_at SET DEFAULT now(),
    ALTER COLUMN last_read_at SET NOT NULL,
    ALTER COLUMN last_read_message_id SET NOT NULL;

ALTER TABLE messages
    DROP COLUMN mention_all;

COMMENT ON COLUMN conversation_user_states.last_read_message_id IS '最后已读消息编号';
COMMENT ON COLUMN conversation_user_states.last_read_at IS '最后标记已读时间';
