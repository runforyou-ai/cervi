-- +goose Up
ALTER TABLE conversations ADD COLUMN last_internal_activity_at timestamptz;

COMMENT ON COLUMN conversations.last_activity_at IS '会话各方可见消息的最后追加活动时间';
COMMENT ON COLUMN conversations.last_internal_activity_at IS '服务会话内部消息的最后追加活动时间';

-- +goose Down
ALTER TABLE conversations DROP COLUMN last_internal_activity_at;

COMMENT ON COLUMN conversations.last_activity_at IS '会话最后消息追加活动时间';
