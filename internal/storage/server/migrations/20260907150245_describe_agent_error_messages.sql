-- +goose Up
COMMENT ON COLUMN messages.type IS '消息类型：text、system、agent_error';
COMMENT ON COLUMN agent_runs.response_message_id IS '运行结果消息编号，关联成功回复或失败消息';

-- +goose Down
COMMENT ON COLUMN messages.type IS '消息类型：text、system';
COMMENT ON COLUMN agent_runs.response_message_id IS '最终回复消息编号';
