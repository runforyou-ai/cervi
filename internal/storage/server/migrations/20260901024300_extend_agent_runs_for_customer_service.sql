-- +goose Up
-- 扩展 Agent 运行账本以区分内部单聊与客户自动接待。
ALTER TABLE conversation_agent_states
    DROP CONSTRAINT conversation_agent_states_pkey,
    ADD PRIMARY KEY (conversation_id, agent_identity_id);

ALTER TABLE conversation_agent_triggers
    ADD COLUMN trigger_type text NOT NULL DEFAULT 'agent_direct',
    ADD COLUMN service_session_id uuid;

ALTER TABLE conversation_agent_triggers
    ALTER COLUMN trigger_type DROP DEFAULT;

DROP INDEX conversation_agent_triggers_conversation_agent_message_unique;

COMMENT ON COLUMN conversation_agent_triggers.trigger_type IS '触发类型：agent_direct、customer_auto';
COMMENT ON COLUMN conversation_agent_triggers.service_session_id IS '客户自动接待所属客服处理周期编号';

ALTER TABLE agent_runs
    ADD COLUMN trigger_type text NOT NULL DEFAULT 'agent_direct',
    ADD COLUMN service_session_id uuid,
    ADD COLUMN error_code text;

ALTER TABLE agent_runs
    ALTER COLUMN trigger_type DROP DEFAULT;

COMMENT ON COLUMN agent_runs.trigger_type IS '触发类型：agent_direct、customer_auto';
COMMENT ON COLUMN agent_runs.service_session_id IS '客户自动接待所属客服处理周期编号';
COMMENT ON COLUMN agent_runs.error_code IS '取消或失败的稳定错误码';
COMMENT ON COLUMN agent_runs.status IS '运行状态：queued、running、succeeded、failed、cancelled';

-- +goose Down
ALTER TABLE agent_runs
    DROP COLUMN error_code,
    DROP COLUMN service_session_id,
    DROP COLUMN trigger_type;

COMMENT ON COLUMN agent_runs.status IS '运行状态：queued、running、succeeded、failed';

CREATE UNIQUE INDEX conversation_agent_triggers_conversation_agent_message_unique
    ON conversation_agent_triggers (conversation_id, agent_identity_id, trigger_message_id);

ALTER TABLE conversation_agent_triggers
    DROP COLUMN service_session_id,
    DROP COLUMN trigger_type;

ALTER TABLE conversation_agent_states
    DROP CONSTRAINT conversation_agent_states_pkey,
    ADD PRIMARY KEY (conversation_id);
