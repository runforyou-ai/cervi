-- +goose Up
-- 创建企业身份表。
CREATE TABLE organization_identities (
    id                      uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    organization_id         uuid NOT NULL,
    type                    text NOT NULL,
    role_id                 uuid NOT NULL,
    display_name            text NOT NULL,
    avatar_file_id          uuid,
    work_status             text NOT NULL DEFAULT 'working',
    work_status_updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX organization_identities_organization_type_index
    ON organization_identities (organization_id, type);

CREATE INDEX organization_identities_organization_name_index
    ON organization_identities (organization_id, lower(display_name));

CREATE INDEX organization_identities_organization_role_index
    ON organization_identities (organization_id, role_id);

COMMENT ON TABLE organization_identities IS '企业身份';
COMMENT ON COLUMN organization_identities.id IS '身份编号';
COMMENT ON COLUMN organization_identities.created_at IS '创建时间';
COMMENT ON COLUMN organization_identities.updated_at IS '更新时间';
COMMENT ON COLUMN organization_identities.organization_id IS '所属企业编号';
COMMENT ON COLUMN organization_identities.type IS '身份类型';
COMMENT ON COLUMN organization_identities.role_id IS '企业角色编号';
COMMENT ON COLUMN organization_identities.display_name IS '显示名称';
COMMENT ON COLUMN organization_identities.avatar_file_id IS '头像文件编号';
COMMENT ON COLUMN organization_identities.work_status IS '工作状态';
COMMENT ON COLUMN organization_identities.work_status_updated_at IS '工作状态更新时间';

-- +goose Down
DROP TABLE organization_identities;
