-- +goose Up
-- 为 Agent 运行增加设备执行位置与租约。
ALTER TABLE agent_runs
    ADD COLUMN execution_device_id    uuid,
    ADD COLUMN execution_workspace_id uuid,
    ADD COLUMN claimed_at             timestamptz,
    ADD COLUMN lease_expires_at       timestamptz;

CREATE UNIQUE INDEX agent_runs_running_workspace_unique
    ON agent_runs (organization_id, execution_workspace_id)
    WHERE execution_workspace_id IS NOT NULL AND status = 'running';

COMMENT ON COLUMN agent_runs.execution_device_id IS '执行设备编号，为空表示服务端执行';
COMMENT ON COLUMN agent_runs.execution_workspace_id IS '执行工作区编号，派发时按会话绑定写入';
COMMENT ON COLUMN agent_runs.claimed_at IS '设备领取时间';
COMMENT ON COLUMN agent_runs.lease_expires_at IS '设备当前租约过期时间';

-- +goose Down
DROP INDEX agent_runs_running_workspace_unique;

ALTER TABLE agent_runs
    DROP COLUMN lease_expires_at,
    DROP COLUMN claimed_at,
    DROP COLUMN execution_workspace_id,
    DROP COLUMN execution_device_id;
