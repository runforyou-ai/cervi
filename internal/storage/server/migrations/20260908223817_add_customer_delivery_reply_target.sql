-- +goose Up
ALTER TABLE customer_message_deliveries ADD COLUMN reply_provider_message_id text;
COMMENT ON COLUMN customer_message_deliveries.reply_provider_message_id IS '入队时确定的引用平台消息编号';

-- +goose Down
ALTER TABLE customer_message_deliveries DROP COLUMN reply_provider_message_id;
