-- +goose Up
-- 桌面端登录会话改为账号会话，旧的成员令牌不再有效；organization_id 与 user_id 记录当前选择的工作区成员身份，未选择时为空。
DELETE FROM client_sessions;
ALTER TABLE client_sessions ADD COLUMN account_id text NOT NULL DEFAULT '';

-- +goose Down
DELETE FROM client_sessions;
ALTER TABLE client_sessions DROP COLUMN account_id;
