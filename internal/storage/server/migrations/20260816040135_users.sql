-- +goose Up
-- 创建企业成员表，关联关系由 Action 维护。
CREATE TABLE users (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    organization_id  uuid NOT NULL,
    email            text NOT NULL,
    display_name     text NOT NULL,
    password_hash    text NOT NULL,
    role             text NOT NULL DEFAULT 'member',
    status           text NOT NULL DEFAULT 'active',
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE users IS '企业成员';
COMMENT ON COLUMN users.id IS '成员编号';
COMMENT ON COLUMN users.organization_id IS '所属企业编号';
COMMENT ON COLUMN users.email IS '登录邮箱';
COMMENT ON COLUMN users.display_name IS '显示名称';
COMMENT ON COLUMN users.password_hash IS '登录密码哈希';
COMMENT ON COLUMN users.role IS '企业角色';
COMMENT ON COLUMN users.status IS '成员状态';
COMMENT ON COLUMN users.created_at IS '创建时间';
COMMENT ON COLUMN users.updated_at IS '更新时间';

CREATE UNIQUE INDEX users_organization_email_unique
    ON users (organization_id, lower(email));

-- +goose Down
DROP TABLE users;
