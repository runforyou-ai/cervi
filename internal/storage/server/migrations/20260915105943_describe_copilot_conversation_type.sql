-- +goose Up
COMMENT ON COLUMN conversations.type IS '会话类型：direct、group、agent、customer、copilot';
COMMENT ON COLUMN agent_inputs.kind IS '输入入口：mention、handoff、agent_direct、customer_auto、copilot';

-- +goose Down
COMMENT ON COLUMN conversations.type IS '会话类型：direct、group、agent、customer';
COMMENT ON COLUMN agent_inputs.kind IS '输入入口：mention、agent_direct、customer_auto';
