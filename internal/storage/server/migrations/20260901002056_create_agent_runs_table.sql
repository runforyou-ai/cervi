-- +goose Up
-- 创建 Agent 业务运行表。
CREATE TABLE agent_runs (
    id                   uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    organization_id      uuid NOT NULL,
    conversation_id      uuid NOT NULL,
    agent_identity_id    uuid NOT NULL,
    agent_revision_id    uuid NOT NULL,
    lane_id              uuid NOT NULL,
    scope_kind           text NOT NULL,
    scope_id             uuid NOT NULL,
    status               text NOT NULL,
    input_start_seq      bigint NOT NULL,
    input_end_seq        bigint,
    response_message_id  uuid,
    behavior_snapshot    jsonb,
    usage                jsonb NOT NULL DEFAULT '{}'::jsonb,
    outcome              text,
    outcome_reason       text,
    handoff_settled_seq  bigint,
    error_code           text,
    last_error           text,
    started_at           timestamptz,
    completed_at         timestamptz
);

CREATE UNIQUE INDEX agent_runs_active_scope_unique
    ON agent_runs (organization_id, scope_kind, scope_id)
    WHERE status IN ('queued', 'running');

COMMENT ON TABLE agent_runs IS 'Agent 业务运行';
COMMENT ON COLUMN agent_runs.id IS '运行编号';
COMMENT ON COLUMN agent_runs.created_at IS '创建时间';
COMMENT ON COLUMN agent_runs.updated_at IS '更新时间';
COMMENT ON COLUMN agent_runs.organization_id IS '所属企业编号';
COMMENT ON COLUMN agent_runs.conversation_id IS '单聊会话编号';
COMMENT ON COLUMN agent_runs.agent_identity_id IS '执行 Agent 企业身份编号';
COMMENT ON COLUMN agent_runs.agent_revision_id IS '运行锁定的 Agent 配置版本';
COMMENT ON COLUMN agent_runs.lane_id IS '本次运行消费的输入队列编号';
COMMENT ON COLUMN agent_runs.scope_kind IS '执行范围类型，与所属队列一致';
COMMENT ON COLUMN agent_runs.scope_id IS '执行范围编号，与所属队列一致';
COMMENT ON COLUMN agent_runs.status IS '运行状态：queued、running、succeeded、failed、cancelled';
COMMENT ON COLUMN agent_runs.input_start_seq IS '本次运行起始输入序号';
COMMENT ON COLUMN agent_runs.input_end_seq IS '本次运行实际消费的最后输入序号';
COMMENT ON COLUMN agent_runs.response_message_id IS '运行结果消息编号，关联成功回复、失败或主动停止消息';
COMMENT ON COLUMN agent_runs.behavior_snapshot IS '运行行为快照：角色基线、场景、规则版本、拼接完成的指令及其哈希、模型参数与工具清单，首次解析时写入并固定';
COMMENT ON COLUMN agent_runs.usage IS '模型用量汇总';
COMMENT ON COLUMN agent_runs.outcome IS '运行结果类型：reply 回答、ask_customer 追问客户、handoff 转交人工；未结束或被取消时为空';
COMMENT ON COLUMN agent_runs.outcome_reason IS '转交人工的原因：model_requested、insufficient_evidence、budget_exhausted、invalid_output、runtime_failed、timeout';
COMMENT ON COLUMN agent_runs.handoff_settled_seq IS '转交人工时原 AI 员工输入队列结算到的输入序号，此前未认领的输入由人工处理';
COMMENT ON COLUMN agent_runs.error_code IS '取消或失败的稳定错误码';
COMMENT ON COLUMN agent_runs.last_error IS '最终失败信息';
COMMENT ON COLUMN agent_runs.started_at IS '首次开始执行时间';
COMMENT ON COLUMN agent_runs.completed_at IS '最终完成时间';

-- +goose Down
DROP TABLE agent_runs;
