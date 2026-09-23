-- +goose Up
-- 删除本机 Agent 工作区路径表，设备运行在会话默认文件夹中执行。
DROP TABLE agent_workspaces;

-- +goose Down
CREATE TABLE agent_workspaces (
    workspace_id    text PRIMARY KEY,
    server_url      text NOT NULL,
    organization_id text NOT NULL,
    path            text NOT NULL,
    created_at      text NOT NULL
);
