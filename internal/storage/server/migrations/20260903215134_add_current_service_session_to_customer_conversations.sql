-- +goose Up
-- 为客户会话记录当前客服处理周期。
ALTER TABLE customer_conversations
    ADD COLUMN current_service_session_id uuid;

COMMENT ON COLUMN customer_conversations.current_service_session_id IS '当前客服处理周期编号';

-- +goose Down
ALTER TABLE customer_conversations
    DROP COLUMN current_service_session_id;
