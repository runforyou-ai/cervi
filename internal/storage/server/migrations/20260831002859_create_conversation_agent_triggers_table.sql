-- +goose Up
-- 创建会话 Agent 用户输入触发表。
CREATE TABLE conversation_agent_triggers (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at          timestamptz NOT NULL DEFAULT now(),
    organization_id     uuid NOT NULL,
    conversation_id     uuid NOT NULL,
    agent_identity_id   uuid NOT NULL,
    trigger_seq         bigint NOT NULL,
    trigger_message_id  uuid NOT NULL,
    agent_run_id        uuid
);

COMMENT ON TABLE conversation_agent_triggers IS '会话 Agent 用户输入触发记录';
COMMENT ON COLUMN conversation_agent_triggers.id IS '触发记录编号';
COMMENT ON COLUMN conversation_agent_triggers.created_at IS '创建时间';
COMMENT ON COLUMN conversation_agent_triggers.organization_id IS '所属企业编号';
COMMENT ON COLUMN conversation_agent_triggers.conversation_id IS '会话编号';
COMMENT ON COLUMN conversation_agent_triggers.agent_identity_id IS '目标 Agent 企业身份编号';
COMMENT ON COLUMN conversation_agent_triggers.trigger_seq IS '会话 Agent 的连续输入序号';
COMMENT ON COLUMN conversation_agent_triggers.trigger_message_id IS '触发本次输入的用户消息编号';
COMMENT ON COLUMN conversation_agent_triggers.agent_run_id IS '实际消费本次输入的 Agent 运行编号';

CREATE UNIQUE INDEX conversation_agent_triggers_conversation_agent_seq_unique
    ON conversation_agent_triggers (conversation_id, agent_identity_id, trigger_seq);

CREATE UNIQUE INDEX conversation_agent_triggers_conversation_agent_message_unique
    ON conversation_agent_triggers (conversation_id, agent_identity_id, trigger_message_id);

CREATE INDEX conversation_agent_triggers_run_seq_index
    ON conversation_agent_triggers (agent_run_id, trigger_seq)
    WHERE agent_run_id IS NOT NULL;

-- +goose Down
DROP TABLE conversation_agent_triggers;
