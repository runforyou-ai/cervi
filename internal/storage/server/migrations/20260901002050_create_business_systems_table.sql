-- +goose Up
-- 创建企业业务系统表。
CREATE TABLE business_systems (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    organization_id  uuid NOT NULL,
    name             text NOT NULL,
    description      text NOT NULL DEFAULT '',
    url              text NOT NULL,
    enabled          boolean NOT NULL DEFAULT true
);

CREATE UNIQUE INDEX business_systems_organization_name_unique
    ON business_systems (organization_id, lower(name));

COMMENT ON TABLE business_systems IS '企业业务系统';
COMMENT ON COLUMN business_systems.id IS '业务系统编号';
COMMENT ON COLUMN business_systems.created_at IS '添加时间';
COMMENT ON COLUMN business_systems.updated_at IS '更新时间';
COMMENT ON COLUMN business_systems.organization_id IS '所属企业编号';
COMMENT ON COLUMN business_systems.name IS '业务系统名称';
COMMENT ON COLUMN business_systems.description IS '业务系统描述';
COMMENT ON COLUMN business_systems.url IS '业务系统地址';
COMMENT ON COLUMN business_systems.enabled IS '是否启用';

-- +goose Down
DROP TABLE business_systems;
