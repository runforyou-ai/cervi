-- +goose Up
-- 为 devices 增加设备上报的可用本机 Agent。
ALTER TABLE devices
    ADD COLUMN local_agents jsonb NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN devices.local_agents IS '设备上报的已安装且可用的本机 Agent 种类';

-- +goose Down
ALTER TABLE devices
    DROP COLUMN local_agents;
