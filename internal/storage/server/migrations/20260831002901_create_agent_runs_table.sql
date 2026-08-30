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
    status               text NOT NULL,
    trigger_start_seq    bigint NOT NULL,
    trigger_end_seq      bigint,
    response_message_id  uuid,
    usage                jsonb NOT NULL DEFAULT '{}'::jsonb,
    last_error           text,
    started_at           timestamptz,
    completed_at         timestamptz
);

COMMENT ON TABLE agent_runs IS 'Agent 业务运行';
COMMENT ON COLUMN agent_runs.id IS '运行编号';
COMMENT ON COLUMN agent_runs.created_at IS '创建时间';
COMMENT ON COLUMN agent_runs.updated_at IS '更新时间';
COMMENT ON COLUMN agent_runs.organization_id IS '所属企业编号';
COMMENT ON COLUMN agent_runs.conversation_id IS '单聊会话编号';
COMMENT ON COLUMN agent_runs.agent_identity_id IS '执行 Agent 企业身份编号';
COMMENT ON COLUMN agent_runs.agent_revision_id IS '运行锁定的 Agent 配置版本';
COMMENT ON COLUMN agent_runs.status IS '运行状态：queued、running、succeeded、failed';
COMMENT ON COLUMN agent_runs.trigger_start_seq IS '本次运行起始输入序号';
COMMENT ON COLUMN agent_runs.trigger_end_seq IS '本次运行实际消费的最后输入序号';
COMMENT ON COLUMN agent_runs.response_message_id IS '最终回复消息编号';
COMMENT ON COLUMN agent_runs.usage IS '模型用量汇总';
COMMENT ON COLUMN agent_runs.last_error IS '最终失败信息';
COMMENT ON COLUMN agent_runs.started_at IS '首次开始执行时间';
COMMENT ON COLUMN agent_runs.completed_at IS '最终完成时间';

CREATE UNIQUE INDEX agent_runs_conversation_active_unique
    ON agent_runs (conversation_id, agent_identity_id)
    WHERE status IN ('queued', 'running');

CREATE INDEX agent_runs_org_conversation_created_index
    ON agent_runs (organization_id, conversation_id, created_at DESC);

-- +goose Down
DROP TABLE agent_runs;
