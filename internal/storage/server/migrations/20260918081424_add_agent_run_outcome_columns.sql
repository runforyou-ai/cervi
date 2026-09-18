-- +goose Up
ALTER TABLE agent_runs
    ADD COLUMN outcome text,
    ADD COLUMN outcome_reason text,
    ADD COLUMN handoff_settled_seq bigint;

COMMENT ON COLUMN agent_runs.outcome IS '运行结果类型：reply 回答、ask_customer 追问客户、handoff 转交人工；未结束或被取消时为空';
COMMENT ON COLUMN agent_runs.outcome_reason IS '转交人工的原因：model_requested、insufficient_evidence、budget_exhausted、runtime_failed、timeout';
COMMENT ON COLUMN agent_runs.handoff_settled_seq IS '转交人工时原 AI 员工输入队列结算到的输入序号，此前未认领的输入由人工处理';

-- +goose Down
ALTER TABLE agent_runs
    DROP COLUMN handoff_settled_seq,
    DROP COLUMN outcome_reason,
    DROP COLUMN outcome;
