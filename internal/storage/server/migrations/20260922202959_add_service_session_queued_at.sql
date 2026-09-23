-- +goose Up
-- 客服处理周期增加进入当前队列的时间。
ALTER TABLE service_sessions
    ADD COLUMN queued_at timestamptz;

COMMENT ON COLUMN service_sessions.queued_at IS '周期进入当前队列的时间，开放且无负责人时有值，有负责人或已关闭时为空';

-- +goose Down
ALTER TABLE service_sessions
    DROP COLUMN queued_at;
