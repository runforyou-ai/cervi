-- +goose Up
-- 创建会话 Agent 输入状态表。
CREATE TABLE conversation_agent_states (
    conversation_id    uuid PRIMARY KEY,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    organization_id    uuid NOT NULL,
    agent_identity_id  uuid NOT NULL,
    desired_seq        bigint NOT NULL DEFAULT 0,
    processed_seq      bigint NOT NULL DEFAULT 0
);

CREATE INDEX conversation_agent_states_organization_agent_updated_index
    ON conversation_agent_states (organization_id, agent_identity_id, updated_at DESC);

COMMENT ON TABLE conversation_agent_states IS '会话 Agent 输入序号状态';
COMMENT ON COLUMN conversation_agent_states.conversation_id IS '会话编号';
COMMENT ON COLUMN conversation_agent_states.created_at IS '创建时间';
COMMENT ON COLUMN conversation_agent_states.updated_at IS '更新时间';
COMMENT ON COLUMN conversation_agent_states.organization_id IS '所属企业编号';
COMMENT ON COLUMN conversation_agent_states.agent_identity_id IS '目标 Agent 企业身份编号';
COMMENT ON COLUMN conversation_agent_states.desired_seq IS '已经持久化的最新输入序号';
COMMENT ON COLUMN conversation_agent_states.processed_seq IS '已经得到明确终态的输入序号';

-- +goose Down
DROP TABLE conversation_agent_states;
