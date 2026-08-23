-- +goose Up
-- 创建用户账号表。
CREATE TABLE users (
    id                      uuid PRIMARY KEY DEFAULT uuidv7(),
    identity_id             uuid NOT NULL,
    organization_id         uuid NOT NULL,
    email                   text NOT NULL,
    password_hash           text NOT NULL,
    role_id                 uuid NOT NULL,
    status                  text NOT NULL DEFAULT 'active',
    locale                  text NOT NULL DEFAULT 'zh-CN',
    time_zone               text NOT NULL DEFAULT 'Asia/Shanghai',
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE users IS '用户账号';
COMMENT ON COLUMN users.id IS '用户编号';
COMMENT ON COLUMN users.identity_id IS '企业身份编号';
COMMENT ON COLUMN users.organization_id IS '所属企业编号';
COMMENT ON COLUMN users.email IS '登录邮箱';
COMMENT ON COLUMN users.password_hash IS '登录密码哈希';
COMMENT ON COLUMN users.role_id IS '企业角色编号';
COMMENT ON COLUMN users.status IS '账号状态';
COMMENT ON COLUMN users.locale IS '界面语言';
COMMENT ON COLUMN users.time_zone IS '日期时间显示时区';
COMMENT ON COLUMN users.created_at IS '创建时间';
COMMENT ON COLUMN users.updated_at IS '更新时间';

CREATE UNIQUE INDEX users_organization_email_unique
    ON users (organization_id, lower(email));

CREATE UNIQUE INDEX users_organization_identity_unique
    ON users (organization_id, identity_id);

CREATE INDEX users_organization_role_index
    ON users (organization_id, role_id);

-- +goose Down
DROP TABLE users;
