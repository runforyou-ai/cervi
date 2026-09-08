-- +goose Up
ALTER TABLE conversations ADD COLUMN last_activity_at timestamptz;
COMMENT ON COLUMN conversations.last_activity_at IS '会话最后消息追加活动时间';

-- +goose Down
ALTER TABLE conversations DROP COLUMN last_activity_at;
