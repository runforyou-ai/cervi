-- +goose Up
-- 创建桌面端应用设置表。
CREATE TABLE app_settings (
    key         text PRIMARY KEY,
    value       text NOT NULL,
    updated_at  text NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE app_settings;
