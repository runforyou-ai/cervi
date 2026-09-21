-- +goose Up
-- 创建本机 Agent 工作区路径表。
CREATE TABLE agent_workspaces (
    workspace_id    text PRIMARY KEY,
    server_url      text NOT NULL,
    organization_id text NOT NULL,
    path            text NOT NULL,
    created_at      text NOT NULL
);

-- +goose Down
DROP TABLE agent_workspaces;
