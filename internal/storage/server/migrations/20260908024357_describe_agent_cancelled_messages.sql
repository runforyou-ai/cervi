-- +goose Up
COMMENT ON COLUMN messages.type IS '消息类型：text、system、agent_error、agent_cancelled';
COMMENT ON COLUMN agent_runs.response_message_id IS '运行结果消息编号，关联成功回复、失败或主动停止消息';

-- +goose Down
COMMENT ON COLUMN messages.type IS '消息类型：text、system、agent_error';
COMMENT ON COLUMN agent_runs.response_message_id IS '运行结果消息编号，关联成功回复或失败消息';
