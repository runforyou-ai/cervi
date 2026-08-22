-- +goose Up
-- 创建企业成员表。
CREATE TABLE organization_members (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    organization_id     uuid NOT NULL,
    type                text NOT NULL,
    display_name        text NOT NULL,
    avatar_file_id      uuid,
    status              text NOT NULL DEFAULT 'active',
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE organization_members IS '企业成员';
COMMENT ON COLUMN organization_members.id IS '成员编号';
COMMENT ON COLUMN organization_members.organization_id IS '所属企业编号';
COMMENT ON COLUMN organization_members.type IS '成员类型';
COMMENT ON COLUMN organization_members.display_name IS '显示名称';
COMMENT ON COLUMN organization_members.avatar_file_id IS '头像文件编号';
COMMENT ON COLUMN organization_members.status IS '成员状态';
COMMENT ON COLUMN organization_members.created_at IS '创建时间';
COMMENT ON COLUMN organization_members.updated_at IS '更新时间';

CREATE INDEX organization_members_organization_type_status_index
    ON organization_members (organization_id, type, status);

CREATE INDEX organization_members_organization_name_index
    ON organization_members (organization_id, lower(display_name));

-- +goose Down
DROP TABLE organization_members;
