-- +goose Up
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

-- +goose Down
DROP TABLE integration_connections;
