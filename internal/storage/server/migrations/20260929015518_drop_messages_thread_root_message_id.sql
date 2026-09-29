-- +goose Up
ALTER TABLE messages DROP COLUMN thread_root_message_id;

COMMENT ON COLUMN agents.handoff_team_id IS '单聊转人工的团队编号，为空时进入公共队列';
COMMENT ON COLUMN service_conversations.source IS '来源：channel 渠道、direct 单聊';

-- +goose Down
ALTER TABLE messages ADD COLUMN thread_root_message_id uuid;

COMMENT ON COLUMN messages.thread_root_message_id IS '讨论串根消息编号';
COMMENT ON COLUMN agents.handoff_team_id IS '单聊与群话题转人工的团队编号，为空时进入公共队列';
COMMENT ON COLUMN service_conversations.source IS '来源：channel 渠道、direct 单聊、group 群聊';
