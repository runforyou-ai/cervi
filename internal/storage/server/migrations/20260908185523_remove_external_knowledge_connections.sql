-- +goose Up
-- 移除外部知识库映射及其分组，本地知识库继续保留。
DELETE FROM knowledge_groups
WHERE knowledge_base_id IN (
    SELECT id FROM knowledge_bases WHERE integration_connection_id IS NOT NULL
);
DELETE FROM knowledge_bases WHERE integration_connection_id IS NOT NULL;

ALTER TABLE knowledge_bases
    DROP COLUMN integration_connection_id,
    DROP COLUMN external_resource_id;
DROP TABLE integration_connections;

-- +goose Down
-- 创建企业外部系统连接器表。
CREATE TABLE integration_connections (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    organization_id  uuid NOT NULL,
    connector_type   text NOT NULL,
    name             text NOT NULL,
    description      text NOT NULL DEFAULT '',
    configuration    jsonb NOT NULL,
    status           text NOT NULL DEFAULT 'untested',
    last_tested_at   timestamptz
);

CREATE UNIQUE INDEX integration_connections_organization_name_unique
    ON integration_connections (organization_id, lower(name));

COMMENT ON TABLE integration_connections IS '企业外部系统连接器';
COMMENT ON COLUMN integration_connections.id IS '连接编号';
COMMENT ON COLUMN integration_connections.created_at IS '添加时间';
COMMENT ON COLUMN integration_connections.updated_at IS '更新时间';
COMMENT ON COLUMN integration_connections.organization_id IS '所属企业编号';
COMMENT ON COLUMN integration_connections.connector_type IS '连接器类型';
COMMENT ON COLUMN integration_connections.name IS '连接名称';
COMMENT ON COLUMN integration_connections.description IS '连接说明';
COMMENT ON COLUMN integration_connections.configuration IS '连接器配置';
COMMENT ON COLUMN integration_connections.status IS '最近一次连接测试状态';
COMMENT ON COLUMN integration_connections.last_tested_at IS '最近一次连接测试时间';

ALTER TABLE knowledge_bases
    ADD COLUMN integration_connection_id uuid,
    ADD COLUMN external_resource_id text;
CREATE UNIQUE INDEX knowledge_bases_external_resource_unique
    ON knowledge_bases (organization_id, integration_connection_id, external_resource_id)
    WHERE integration_connection_id IS NOT NULL AND external_resource_id IS NOT NULL;
COMMENT ON COLUMN knowledge_bases.integration_connection_id IS '外部知识库使用的集成连接编号';
COMMENT ON COLUMN knowledge_bases.external_resource_id IS '外部平台中的知识库编号';
