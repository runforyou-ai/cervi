-- +goose Up
-- 会话 Agent 输入状态与触发记录已由 agent_lanes 和 agent_inputs 承载。
DROP TABLE conversation_agent_triggers;
DROP TABLE conversation_agent_states;

-- +goose Down
CREATE TABLE conversation_agent_states (
    conversation_id    uuid NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    organization_id    uuid NOT NULL,
    agent_identity_id  uuid NOT NULL,
    desired_seq        bigint NOT NULL DEFAULT 0,
    processed_seq      bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (conversation_id, agent_identity_id)
);

CREATE TABLE conversation_agent_triggers (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at          timestamptz NOT NULL DEFAULT now(),
    organization_id     uuid NOT NULL,
    conversation_id     uuid NOT NULL,
    agent_identity_id   uuid NOT NULL,
    trigger_seq         bigint NOT NULL,
    trigger_message_id  uuid NOT NULL,
    agent_run_id        uuid,
    trigger_type        text NOT NULL,
    service_session_id  uuid
);

CREATE UNIQUE INDEX conversation_agent_triggers_conversation_agent_seq_unique
    ON conversation_agent_triggers (conversation_id, agent_identity_id, trigger_seq);

COMMENT ON TABLE conversation_agent_states IS '会话 Agent 输入序号状态';
COMMENT ON TABLE conversation_agent_triggers IS '会话 Agent 用户输入触发记录';
