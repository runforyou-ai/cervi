-- +goose Up
-- 创建企业 MCP 服务表。
CREATE TABLE mcp_servers (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    organization_id  uuid NOT NULL,
    name             text NOT NULL,
    url              text NOT NULL,
    server_type      text NOT NULL,
    authorization_token text NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX mcp_servers_organization_name_unique
    ON mcp_servers (organization_id, lower(name));

COMMENT ON TABLE mcp_servers IS '企业 MCP 服务';
COMMENT ON COLUMN mcp_servers.id IS 'MCP 服务编号';
COMMENT ON COLUMN mcp_servers.created_at IS '添加时间';
COMMENT ON COLUMN mcp_servers.updated_at IS '更新时间';
COMMENT ON COLUMN mcp_servers.organization_id IS '所属企业编号';
COMMENT ON COLUMN mcp_servers.name IS 'MCP 服务名称';
COMMENT ON COLUMN mcp_servers.url IS 'MCP 服务地址';
COMMENT ON COLUMN mcp_servers.server_type IS '服务器类型';
COMMENT ON COLUMN mcp_servers.authorization_token IS '认证令牌';

-- +goose Down
DROP TABLE mcp_servers;
