-- +goose Up
-- 删除网站渠道聊天窗口副标题。
ALTER TABLE website_channel_settings DROP COLUMN chat_subtitle;

-- +goose Down
ALTER TABLE website_channel_settings ADD COLUMN chat_subtitle text;

COMMENT ON COLUMN website_channel_settings.chat_subtitle IS '聊天窗口副标题';
