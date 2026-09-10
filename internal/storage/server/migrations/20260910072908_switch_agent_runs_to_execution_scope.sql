-- +goose Up
-- Agent 运行按 Lane 与执行范围组织。运行记录是执行事实，结构切换时清空，并一并清理指向这些运行的待执行任务。
TRUNCATE agent_run_blocks, agent_runs;

DELETE FROM task_outbox WHERE task_run_id IN (SELECT id FROM task_runs WHERE action_name = 'agent.run');
DELETE FROM task_runs WHERE action_name = 'agent.run';

ALTER TABLE agent_runs
    ADD COLUMN lane_id uuid NOT NULL,
    ADD COLUMN scope_kind text NOT NULL,
    ADD COLUMN scope_id uuid NOT NULL,
    ADD COLUMN outcome text,
    DROP COLUMN trigger_type,
    DROP COLUMN service_session_id;

ALTER TABLE agent_runs RENAME COLUMN trigger_start_seq TO input_start_seq;
ALTER TABLE agent_runs RENAME COLUMN trigger_end_seq TO input_end_seq;

DROP INDEX agent_runs_conversation_active_unique;

CREATE UNIQUE INDEX agent_runs_active_scope_unique
    ON agent_runs (organization_id, scope_kind, scope_id)
    WHERE status IN ('queued', 'running');

COMMENT ON COLUMN agent_runs.lane_id IS '本次运行消费的输入队列编号';
COMMENT ON COLUMN agent_runs.scope_kind IS '执行范围类型，与所属队列一致';
COMMENT ON COLUMN agent_runs.scope_id IS '执行范围编号，与所属队列一致';
COMMENT ON COLUMN agent_runs.outcome IS '运行结果类型：reply、silent';
COMMENT ON COLUMN agent_runs.input_start_seq IS '本次运行起始输入序号';
COMMENT ON COLUMN agent_runs.input_end_seq IS '本次运行实际消费的最后输入序号';

-- +goose Down
TRUNCATE agent_run_blocks, agent_runs;

DELETE FROM task_outbox WHERE task_run_id IN (SELECT id FROM task_runs WHERE action_name = 'agent.run');
DELETE FROM task_runs WHERE action_name = 'agent.run';

DROP INDEX agent_runs_active_scope_unique;

ALTER TABLE agent_runs RENAME COLUMN input_end_seq TO trigger_end_seq;
ALTER TABLE agent_runs RENAME COLUMN input_start_seq TO trigger_start_seq;

ALTER TABLE agent_runs
    DROP COLUMN outcome,
    DROP COLUMN scope_id,
    DROP COLUMN scope_kind,
    DROP COLUMN lane_id,
    ADD COLUMN trigger_type text NOT NULL,
    ADD COLUMN service_session_id uuid;

CREATE UNIQUE INDEX agent_runs_conversation_active_unique
    ON agent_runs (conversation_id, agent_identity_id)
    WHERE status IN ('queued', 'running');

COMMENT ON COLUMN agent_runs.trigger_type IS '触发类型：agent_direct、customer_auto';
COMMENT ON COLUMN agent_runs.service_session_id IS '客户自动接待所属客服处理周期编号';
COMMENT ON COLUMN agent_runs.trigger_start_seq IS '本次运行起始输入序号';
COMMENT ON COLUMN agent_runs.trigger_end_seq IS '本次运行实际消费的最后输入序号';
