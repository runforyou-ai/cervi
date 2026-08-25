-- +goose Up
-- 创建企业知识库表。
CREATE TABLE knowledge_bases (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    organization_id     uuid NOT NULL,
    created_by_user_id  uuid NOT NULL,
    name                text NOT NULL,
    category            text NOT NULL,
    description         text NOT NULL DEFAULT '',
    integration_connection_id uuid,
    external_resource_id text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE knowledge_bases IS '企业知识库';
COMMENT ON COLUMN knowledge_bases.id IS '知识库编号';
COMMENT ON COLUMN knowledge_bases.organization_id IS '所属企业编号';
COMMENT ON COLUMN knowledge_bases.created_by_user_id IS '创建用户编号';
COMMENT ON COLUMN knowledge_bases.name IS '知识库名称';
COMMENT ON COLUMN knowledge_bases.category IS '知识库类型：standard 文档库、qa 问答库';
COMMENT ON COLUMN knowledge_bases.description IS '知识库描述';
COMMENT ON COLUMN knowledge_bases.integration_connection_id IS '外部知识库使用的集成连接编号';
COMMENT ON COLUMN knowledge_bases.external_resource_id IS '外部平台中的知识库编号';
COMMENT ON COLUMN knowledge_bases.created_at IS '创建时间';
COMMENT ON COLUMN knowledge_bases.updated_at IS '更新时间';

CREATE UNIQUE INDEX knowledge_bases_organization_name_unique
    ON knowledge_bases (organization_id, lower(name));

CREATE UNIQUE INDEX knowledge_bases_external_resource_unique
    ON knowledge_bases (organization_id, integration_connection_id, external_resource_id)
    WHERE integration_connection_id IS NOT NULL AND external_resource_id IS NOT NULL;

-- +goose Down
DROP TABLE knowledge_bases;
