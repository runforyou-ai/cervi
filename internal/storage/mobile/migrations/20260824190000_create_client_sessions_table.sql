-- +goose Up
-- 创建移动端当前登录会话表。
CREATE TABLE client_sessions (
    id              text PRIMARY KEY,
    server_url      text NOT NULL,
    organization_id text NOT NULL,
    user_id         text NOT NULL,
    token           text NOT NULL,
    expires_at      text NOT NULL,
    updated_at      text NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE client_sessions;
