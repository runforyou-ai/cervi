-- +goose Up
CREATE TABLE agent_conversations (
    conversation_id uuid PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    organization_id uuid NOT NULL,
    user_identity_id uuid NOT NULL,
    agent_identity_id uuid NOT NULL
);

COMMENT ON TABLE agent_conversations IS 'AI 聊天业务归属';
COMMENT ON COLUMN agent_conversations.conversation_id IS '会话编号';
COMMENT ON COLUMN agent_conversations.created_at IS '创建时间';
COMMENT ON COLUMN agent_conversations.updated_at IS '更新时间';
COMMENT ON COLUMN agent_conversations.organization_id IS '所属企业编号';
COMMENT ON COLUMN agent_conversations.user_identity_id IS '所属成员身份编号';
COMMENT ON COLUMN agent_conversations.agent_identity_id IS '目标 Agent 身份编号';

-- +goose Down
DROP TABLE agent_conversations;
