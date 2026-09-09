-- +goose Up
CREATE TABLE agent_mcp_servers (
    organization_id uuid NOT NULL,
    agent_id uuid NOT NULL,
    mcp_server_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, agent_id, mcp_server_id)
);

COMMENT ON TABLE agent_mcp_servers IS 'AI 员工当前绑定的 MCP 服务';
COMMENT ON COLUMN agent_mcp_servers.organization_id IS '所属企业编号';
COMMENT ON COLUMN agent_mcp_servers.agent_id IS 'AI 员工编号';
COMMENT ON COLUMN agent_mcp_servers.mcp_server_id IS 'MCP 服务编号';
COMMENT ON COLUMN agent_mcp_servers.created_at IS '绑定创建时间';

-- +goose Down
DROP TABLE agent_mcp_servers;
