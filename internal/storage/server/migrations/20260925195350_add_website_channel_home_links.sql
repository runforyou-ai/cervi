-- +goose Up
-- 增加网站渠道 Messenger 首页链接。
ALTER TABLE website_channel_settings ADD COLUMN home_links jsonb NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN website_channel_settings.home_links IS 'Messenger 首页链接，按展示顺序保存标题与地址';

-- +goose Down
ALTER TABLE website_channel_settings DROP COLUMN home_links;
