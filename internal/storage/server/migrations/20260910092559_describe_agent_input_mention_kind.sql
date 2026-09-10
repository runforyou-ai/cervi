-- +goose Up
-- 群内点名成为持久输入的业务入口之一。
COMMENT ON COLUMN agent_inputs.kind IS '输入入口：mention、agent_direct、customer_auto';

-- +goose Down
COMMENT ON COLUMN agent_inputs.kind IS '输入入口：agent_direct、customer_auto';
