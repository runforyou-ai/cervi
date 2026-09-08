-- +goose Up
ALTER TABLE mcp_servers
    ADD COLUMN tools jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN tools_updated_at timestamptz,
    ADD COLUMN tools_refresh_id uuid,
    ADD COLUMN tools_failure text NOT NULL DEFAULT '';

COMMENT ON COLUMN mcp_servers.tools IS '最近成功获取的工具目录';
COMMENT ON COLUMN mcp_servers.tools_updated_at IS '工具目录更新时间';
COMMENT ON COLUMN mcp_servers.tools_refresh_id IS '当前工具更新批次';
COMMENT ON COLUMN mcp_servers.tools_failure IS '工具更新失败原因码';

-- +goose Down
ALTER TABLE mcp_servers
    DROP COLUMN tools,
    DROP COLUMN tools_updated_at,
    DROP COLUMN tools_refresh_id,
    DROP COLUMN tools_failure;
