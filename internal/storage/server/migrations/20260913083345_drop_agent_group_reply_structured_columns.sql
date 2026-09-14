-- +goose Up
-- 群内 Agent 回复从正文提取点名，删除结构化结果类型与接力深度。
ALTER TABLE agent_runs DROP COLUMN outcome;

ALTER TABLE agent_inputs DROP COLUMN depth;

-- +goose Down
ALTER TABLE agent_runs ADD COLUMN outcome text;

ALTER TABLE agent_inputs ADD COLUMN depth integer NOT NULL DEFAULT 0;

COMMENT ON COLUMN agent_runs.outcome IS '运行结果类型：reply、silent';
COMMENT ON COLUMN agent_inputs.depth IS '接力深度，真人发起为 0';
