-- +goose Up
-- 创建角色权限关联表。
CREATE TABLE role_permissions (
    role_id          uuid NOT NULL,
    permission       text NOT NULL,
    PRIMARY KEY (role_id, permission),

    created_at       timestamptz NOT NULL DEFAULT now(),
    organization_id  uuid NOT NULL
);

COMMENT ON TABLE role_permissions IS '角色权限关联';
COMMENT ON COLUMN role_permissions.role_id IS '角色编号';
COMMENT ON COLUMN role_permissions.permission IS '预定义权限代码';
COMMENT ON COLUMN role_permissions.created_at IS '创建时间';
COMMENT ON COLUMN role_permissions.organization_id IS '所属企业编号';

-- +goose Down
DROP TABLE role_permissions;
