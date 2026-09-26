-- +goose Up
-- 增加网站渠道 Messenger 的页签、首页与对话功能设置。
ALTER TABLE website_channel_settings
    ADD COLUMN home_enabled boolean NOT NULL DEFAULT TRUE,
    ADD COLUMN help_enabled boolean NOT NULL DEFAULT TRUE,
    ADD COLUMN home_welcome text,
    ADD COLUMN home_headline text,
    ADD COLUMN home_blocks jsonb NOT NULL DEFAULT '[{"type":"recent_conversation","enabled":true},{"type":"start_conversation","enabled":true},{"type":"links","enabled":true}]'::jsonb,
    ADD COLUMN home_links jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN attachments_enabled boolean NOT NULL DEFAULT TRUE,
    ADD COLUMN emoji_enabled boolean NOT NULL DEFAULT TRUE,
    ADD COLUMN rating_enabled boolean NOT NULL DEFAULT TRUE;

COMMENT ON COLUMN website_channel_settings.home_enabled IS '是否显示 Messenger 首页页签';
COMMENT ON COLUMN website_channel_settings.help_enabled IS '是否显示 Messenger 帮助页签，同时需要发布知识库';
COMMENT ON COLUMN website_channel_settings.home_welcome IS '首页问候语第一行，为空时使用默认文案';
COMMENT ON COLUMN website_channel_settings.home_headline IS '首页问候语第二行，为空时使用默认文案';
COMMENT ON COLUMN website_channel_settings.home_blocks IS '首页卡片的展示顺序与开关';
COMMENT ON COLUMN website_channel_settings.home_links IS 'Messenger 首页链接，按展示顺序保存标题与地址';
COMMENT ON COLUMN website_channel_settings.attachments_enabled IS '访客是否可以发送附件';
COMMENT ON COLUMN website_channel_settings.emoji_enabled IS '访客输入框是否显示表情';
COMMENT ON COLUMN website_channel_settings.rating_enabled IS '客服周期结束后是否邀请访客评价';

-- +goose Down
ALTER TABLE website_channel_settings
    DROP COLUMN home_enabled,
    DROP COLUMN help_enabled,
    DROP COLUMN home_welcome,
    DROP COLUMN home_headline,
    DROP COLUMN home_blocks,
    DROP COLUMN home_links,
    DROP COLUMN attachments_enabled,
    DROP COLUMN emoji_enabled,
    DROP COLUMN rating_enabled;
