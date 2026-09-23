-- +goose Up
-- 为 agents 增加助理的主人、绑定电脑与暂停状态。
ALTER TABLE agents
    ADD COLUMN owner_user_id uuid,
    ADD COLUMN device_id     uuid,
    ADD COLUMN paused_at     timestamptz;

COMMENT ON TABLE agents IS 'AI 员工与助理，类型以企业身份为准';
COMMENT ON COLUMN agents.id IS 'AI 员工或助理编号';
COMMENT ON COLUMN agents.owner_user_id IS '助理主人编号，AI 员工为空';
COMMENT ON COLUMN agents.device_id IS '助理绑定的电脑编号，AI 员工为空';
COMMENT ON COLUMN agents.paused_at IS '主人暂停助理的时间，非空表示暂停';

-- +goose Down
COMMENT ON TABLE agents IS 'AI 员工';
COMMENT ON COLUMN agents.id IS 'AI 员工编号';

ALTER TABLE agents
    DROP COLUMN paused_at,
    DROP COLUMN device_id,
    DROP COLUMN owner_user_id;
