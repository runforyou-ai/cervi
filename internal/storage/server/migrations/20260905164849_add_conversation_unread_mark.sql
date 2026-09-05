-- +goose Up
ALTER TABLE conversation_user_states ADD COLUMN marked_unread boolean NOT NULL DEFAULT false;
COMMENT ON COLUMN conversation_user_states.marked_unread IS '用户主动设置的独立未读标记';

-- +goose Down
ALTER TABLE conversation_user_states DROP COLUMN marked_unread;
