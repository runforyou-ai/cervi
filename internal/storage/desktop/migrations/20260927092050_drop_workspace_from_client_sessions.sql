-- +goose Up
-- 登录会话只保存账号会话，各请求自行携带目标工作区。
ALTER TABLE client_sessions DROP COLUMN organization_id;
ALTER TABLE client_sessions DROP COLUMN user_id;

-- +goose Down
ALTER TABLE client_sessions ADD COLUMN organization_id text NOT NULL DEFAULT '';
ALTER TABLE client_sessions ADD COLUMN user_id text NOT NULL DEFAULT '';
