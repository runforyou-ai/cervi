-- +goose Up
-- 服务周期记录需要真人、真人首次负责与真人首次对客回复的时间，以及按工作时间计的真人首响用时。
ALTER TABLE service_sessions
    ADD COLUMN human_requested_at timestamptz,
    ADD COLUMN human_assigned_at timestamptz,
    ADD COLUMN human_first_response_at timestamptz,
    ADD COLUMN human_first_response_seconds integer;

COMMENT ON COLUMN service_sessions.human_requested_at IS '周期首次需要真人的时间：真人或队列首接待时为周期开启时间，其余为首次转人工、退回队列或交给真人负责的时间；从未需要真人时为空';
COMMENT ON COLUMN service_sessions.human_assigned_at IS '真人首次负责该周期的时间；从未由真人负责时为空';
COMMENT ON COLUMN service_sessions.human_first_response_at IS '真人首次对客回复的时间；没有真人对客回复时为空';
COMMENT ON COLUMN service_sessions.human_first_response_seconds IS '真人首响用时（秒）：首次需要真人到真人首次对客回复之间落在客服工作时间内的时长，按回复时的工作时间设置计算；没有真人对客回复或其间没有经过工作时间时为空';

-- +goose Down
ALTER TABLE service_sessions
    DROP COLUMN human_requested_at,
    DROP COLUMN human_assigned_at,
    DROP COLUMN human_first_response_at,
    DROP COLUMN human_first_response_seconds;
