-- +goose Up
ALTER TABLE messages ADD COLUMN client_message_id uuid;
COMMENT ON COLUMN messages.client_message_id IS '发送方客户端消息编号';

-- +goose Down
ALTER TABLE messages DROP COLUMN client_message_id;
