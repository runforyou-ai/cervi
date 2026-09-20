-- +goose Up
-- 创建桌面端本机设备在各企业服务器上的注册结果表。
CREATE TABLE device_registrations (
    server_url      text NOT NULL,
    organization_id text NOT NULL,
    user_id         text NOT NULL,
    device_id       text NOT NULL,
    registered_at   text NOT NULL,
    PRIMARY KEY (server_url, organization_id, user_id)
);

-- +goose Down
DROP TABLE device_registrations;
