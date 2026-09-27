-- +goose Up
-- 本机设备按「服务器地址 + 账号 + 工作区」登记注册结果；旧记录按成员登记，删除后由设备注册循环重新注册。
DROP TABLE device_registrations;
CREATE TABLE device_registrations (
    server_url      text NOT NULL,
    account_id      text NOT NULL,
    organization_id text NOT NULL,
    device_id       text NOT NULL,
    registered_at   text NOT NULL,
    PRIMARY KEY (server_url, account_id, organization_id)
);

-- +goose Down
DROP TABLE device_registrations;
CREATE TABLE device_registrations (
    server_url      text NOT NULL,
    organization_id text NOT NULL,
    user_id         text NOT NULL,
    device_id       text NOT NULL,
    registered_at   text NOT NULL,
    PRIMARY KEY (server_url, organization_id, user_id)
);
