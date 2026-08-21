-- +goose Up
-- 为企业成员保存界面语言和日期时间显示时区。
ALTER TABLE users
    ADD COLUMN locale text NOT NULL DEFAULT 'zh-CN',
    ADD COLUMN time_zone text NOT NULL DEFAULT 'Asia/Shanghai';

-- +goose Down
ALTER TABLE users
    DROP COLUMN time_zone,
    DROP COLUMN locale;
