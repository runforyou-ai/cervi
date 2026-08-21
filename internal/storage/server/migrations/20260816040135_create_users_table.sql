-- +goose Up
-- 创建企业成员表。
CREATE TABLE users (
    id                      uuid PRIMARY KEY DEFAULT uuidv7(),
    organization_id         uuid NOT NULL,
    email                   text NOT NULL,
    display_name            text NOT NULL,
    password_hash           text NOT NULL,
    role_id                 uuid NOT NULL,
    status                  text NOT NULL DEFAULT 'active',
    locale                  text NOT NULL DEFAULT 'zh-CN',
    time_zone               text NOT NULL DEFAULT 'Asia/Shanghai',
    work_status             text NOT NULL DEFAULT 'working',
    work_status_updated_at  timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    avatar_file_id          uuid,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE users IS '企业成员';
COMMENT ON COLUMN users.id IS '成员编号';
COMMENT ON COLUMN users.organization_id IS '所属企业编号';
COMMENT ON COLUMN users.email IS '登录邮箱';
COMMENT ON COLUMN users.display_name IS '显示名称';
COMMENT ON COLUMN users.password_hash IS '登录密码哈希';
COMMENT ON COLUMN users.role_id IS '企业角色编号';
COMMENT ON COLUMN users.status IS '成员状态';
COMMENT ON COLUMN users.locale IS '界面语言';
COMMENT ON COLUMN users.time_zone IS '日期时间显示时区';
COMMENT ON COLUMN users.work_status IS '用户主动设置的工作状态';
COMMENT ON COLUMN users.work_status_updated_at IS '工作状态更新时间';
COMMENT ON COLUMN users.avatar_file_id IS '当前头像文件编号';
COMMENT ON COLUMN users.created_at IS '创建时间';
COMMENT ON COLUMN users.updated_at IS '更新时间';

CREATE UNIQUE INDEX users_organization_email_unique
    ON users (organization_id, lower(email));

CREATE INDEX users_organization_role_index
    ON users (organization_id, role_id);

-- +goose Down
DROP TABLE users;
