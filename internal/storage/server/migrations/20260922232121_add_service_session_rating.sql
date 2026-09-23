-- +goose Up
-- 客服处理周期增加访客评价。
ALTER TABLE service_sessions
    ADD COLUMN rating_resolved boolean,
    ADD COLUMN rating_comment text,
    ADD COLUMN rated_at timestamptz;

COMMENT ON COLUMN service_sessions.rating_resolved IS '访客评价的是否解决，周期关闭后由访客提交；未评价时为空';
COMMENT ON COLUMN service_sessions.rating_comment IS '访客评价的评语；未填写时为空字符串，未评价时为空';
COMMENT ON COLUMN service_sessions.rated_at IS '访客提交评价的时间；未评价时为空';

-- +goose Down
ALTER TABLE service_sessions
    DROP COLUMN rated_at,
    DROP COLUMN rating_comment,
    DROP COLUMN rating_resolved;
