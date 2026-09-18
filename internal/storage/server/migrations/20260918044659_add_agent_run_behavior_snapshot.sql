-- +goose Up
ALTER TABLE agent_runs
    ADD COLUMN behavior_snapshot jsonb;

COMMENT ON COLUMN agent_runs.behavior_snapshot IS '运行行为快照：角色基线、场景、规则版本、拼接完成的指令及其哈希、模型参数与工具清单，首次解析时写入并固定';

-- +goose Down
ALTER TABLE agent_runs
    DROP COLUMN behavior_snapshot;
