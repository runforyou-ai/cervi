-- +goose Up
-- 创建设备工作区表。
CREATE TABLE device_workspaces (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    organization_id uuid NOT NULL,
    device_id       uuid NOT NULL,
    label           text NOT NULL,
    last_used_at    timestamptz
);

COMMENT ON TABLE device_workspaces IS '成员设备上供 Agent 执行本机工具的工作区，真实路径只保存在设备本地';
COMMENT ON COLUMN device_workspaces.id IS '工作区编号';
COMMENT ON COLUMN device_workspaces.created_at IS '注册时间';
COMMENT ON COLUMN device_workspaces.updated_at IS '更新时间';
COMMENT ON COLUMN device_workspaces.organization_id IS '所属企业编号';
COMMENT ON COLUMN device_workspaces.device_id IS '所属设备编号';
COMMENT ON COLUMN device_workspaces.label IS '用户可见的工作区显示名';
COMMENT ON COLUMN device_workspaces.last_used_at IS '最近一次领取运行的时间';

-- +goose Down
DROP TABLE device_workspaces;
