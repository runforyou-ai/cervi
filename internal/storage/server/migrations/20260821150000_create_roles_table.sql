-- +goose Up
-- 创建企业角色表。
CREATE TABLE roles (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    organization_id  uuid NOT NULL,
    kind             text NOT NULL,
    name             text NOT NULL DEFAULT '',
    description      text NOT NULL DEFAULT ''
);

COMMENT ON TABLE roles IS '企业角色';
COMMENT ON COLUMN roles.id IS '角色编号';
COMMENT ON COLUMN roles.created_at IS '创建时间';
COMMENT ON COLUMN roles.updated_at IS '更新时间';
COMMENT ON COLUMN roles.organization_id IS '所属企业编号';
COMMENT ON COLUMN roles.kind IS '内置角色类型或自定义角色标识';
COMMENT ON COLUMN roles.name IS '自定义角色名称';
COMMENT ON COLUMN roles.description IS '自定义角色说明';

CREATE UNIQUE INDEX roles_organization_builtin_kind_unique
    ON roles (organization_id, kind)
    WHERE kind <> 'custom';

CREATE UNIQUE INDEX roles_organization_custom_name_unique
    ON roles (organization_id, lower(name))
    WHERE kind = 'custom';

CREATE INDEX roles_organization_created_at_index
    ON roles (organization_id, created_at);

-- +goose Down
DROP TABLE roles;
