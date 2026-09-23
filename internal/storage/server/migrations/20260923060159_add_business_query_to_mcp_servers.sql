-- +goose Up
-- 为 MCP 服务增加工具用途与按客户查询开关。
ALTER TABLE mcp_servers
    ADD COLUMN tool_purposes   jsonb   NOT NULL DEFAULT '{}',
    ADD COLUMN customer_scoped boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN mcp_servers.tool_purposes IS '工具名到用途的映射：query 为查询，action 为操作；未出现的工具为未标记';
COMMENT ON COLUMN mcp_servers.customer_scoped IS '是否按客户查询：开启后客服运行在请求头中附加已验证客户的身份';

-- +goose Down
ALTER TABLE mcp_servers
    DROP COLUMN customer_scoped,
    DROP COLUMN tool_purposes;
