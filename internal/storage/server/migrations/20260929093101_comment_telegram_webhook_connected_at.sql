-- +goose Up
-- 更新 Webhook 连接时间的字段说明。
COMMENT ON COLUMN telegram_channel_settings.webhook_connected_at IS '当前 Webhook 注册后首次成功回调的时间，重新注册或停用时清空';

-- +goose Down
COMMENT ON COLUMN telegram_channel_settings.webhook_connected_at IS 'Webhook 最近连接时间';
