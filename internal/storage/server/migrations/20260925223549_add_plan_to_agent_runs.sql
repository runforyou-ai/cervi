-- +goose Up
ALTER TABLE agent_runs ADD COLUMN plan jsonb;

COMMENT ON COLUMN agent_runs.plan IS '运行结束时的任务清单，为空表示没有建立清单';

-- +goose Down
ALTER TABLE agent_runs DROP COLUMN plan;
