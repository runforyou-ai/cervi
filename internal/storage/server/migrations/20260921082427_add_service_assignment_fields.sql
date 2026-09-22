-- +goose Up
-- 成员增加最大接待量与最近自动分配时间，客服处理周期增加当前负责人接手时间与客户等待回复起点。
ALTER TABLE users
    ADD COLUMN max_service_sessions integer NOT NULL DEFAULT 10,
    ADD COLUMN last_service_assigned_at timestamptz;

ALTER TABLE service_sessions
    ADD COLUMN assignee_assigned_at timestamptz,
    ADD COLUMN awaiting_reply_since timestamptz;

COMMENT ON COLUMN users.max_service_sessions IS '最大接待量：自动分配时本人可负责的开放客服处理周期上限';
COMMENT ON COLUMN users.last_service_assigned_at IS '最近一次被自动分配客服处理周期的时间';
COMMENT ON COLUMN service_sessions.assignee_assigned_at IS '当前负责人获得本周期的时间，周期在队列中时为空';
COMMENT ON COLUMN service_sessions.awaiting_reply_since IS '客户开始等待回复的时间，为空表示没有待回复的客户消息';

-- +goose Down
ALTER TABLE service_sessions
    DROP COLUMN awaiting_reply_since,
    DROP COLUMN assignee_assigned_at;

ALTER TABLE users
    DROP COLUMN last_service_assigned_at,
    DROP COLUMN max_service_sessions;
