-- +goose Up
-- 用户本地密码改为可空，空值表示该用户没有本地密码。
ALTER TABLE users
    ALTER COLUMN password_hash DROP NOT NULL;

COMMENT ON COLUMN users.password_hash IS '本地登录密码哈希，为空表示没有本地密码';

-- +goose Down
UPDATE users SET password_hash = '' WHERE password_hash IS NULL;

ALTER TABLE users
    ALTER COLUMN password_hash SET NOT NULL;

COMMENT ON COLUMN users.password_hash IS '登录密码哈希';
