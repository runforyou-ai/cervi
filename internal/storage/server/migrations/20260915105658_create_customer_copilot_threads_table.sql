-- +goose Up
CREATE TABLE customer_copilot_threads (
    conversation_id uuid PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    organization_id uuid NOT NULL,
    customer_conversation_id uuid NOT NULL,
    agent_identity_id uuid NOT NULL,
    created_by_identity_id uuid NOT NULL
);

COMMENT ON TABLE customer_copilot_threads IS '客户会话 Copilot 线程归属';
COMMENT ON COLUMN customer_copilot_threads.conversation_id IS 'Copilot 线程会话编号';
COMMENT ON COLUMN customer_copilot_threads.created_at IS '创建时间';
COMMENT ON COLUMN customer_copilot_threads.updated_at IS '更新时间';
COMMENT ON COLUMN customer_copilot_threads.organization_id IS '所属企业编号';
COMMENT ON COLUMN customer_copilot_threads.customer_conversation_id IS '所属客户会话编号';
COMMENT ON COLUMN customer_copilot_threads.agent_identity_id IS '回答问题的 AI 员工身份编号';
COMMENT ON COLUMN customer_copilot_threads.created_by_identity_id IS '创建线程的成员身份编号';

-- +goose Down
DROP TABLE customer_copilot_threads;
