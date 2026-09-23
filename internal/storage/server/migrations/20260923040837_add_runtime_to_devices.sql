-- +goose Up
-- 为设备增加本机运行时版本与本机工具清单。
ALTER TABLE devices
    ADD COLUMN runtime_version integer NOT NULL DEFAULT 0,
    ADD COLUMN tool_manifest   jsonb   NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN devices.runtime_version IS '设备最近一次注册上报的本机运行时版本';
COMMENT ON COLUMN devices.tool_manifest IS '设备最近一次注册上报的本机工具名称清单';

-- +goose Down
ALTER TABLE devices
    DROP COLUMN tool_manifest,
    DROP COLUMN runtime_version;
