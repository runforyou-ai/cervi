-- +goose Up
-- 删除助理工作区与设备本机运行时协商，设备运行在会话默认文件夹中执行。
DROP INDEX agent_runs_running_workspace_unique;

ALTER TABLE agent_runs DROP COLUMN execution_workspace_id;

DROP TABLE conversation_assistant_workspaces;

DROP TABLE device_workspaces;

ALTER TABLE devices
    DROP COLUMN tool_manifest,
    DROP COLUMN runtime_version;

-- +goose Down
ALTER TABLE devices
    ADD COLUMN runtime_version integer NOT NULL DEFAULT 0,
    ADD COLUMN tool_manifest   jsonb   NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN devices.runtime_version IS '设备最近一次注册上报的本机运行时版本';
COMMENT ON COLUMN devices.tool_manifest IS '设备最近一次注册上报的本机工具名称清单';

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

CREATE TABLE conversation_assistant_workspaces (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    organization_id     uuid NOT NULL,
    conversation_id     uuid NOT NULL,
    agent_id            uuid NOT NULL,
    workspace_id        uuid NOT NULL,
    assigned_by_user_id uuid NOT NULL
);

CREATE UNIQUE INDEX conversation_assistant_workspaces_conversation_agent_unique
    ON conversation_assistant_workspaces (organization_id, conversation_id, agent_id);

COMMENT ON TABLE conversation_assistant_workspaces IS '主人为助理在会话中指定的本机工作区';
COMMENT ON COLUMN conversation_assistant_workspaces.id IS '指定记录编号';
COMMENT ON COLUMN conversation_assistant_workspaces.created_at IS '创建时间';
COMMENT ON COLUMN conversation_assistant_workspaces.updated_at IS '更新时间';
COMMENT ON COLUMN conversation_assistant_workspaces.organization_id IS '所属企业编号';
COMMENT ON COLUMN conversation_assistant_workspaces.conversation_id IS '会话编号';
COMMENT ON COLUMN conversation_assistant_workspaces.agent_id IS '助理编号';
COMMENT ON COLUMN conversation_assistant_workspaces.workspace_id IS '助理绑定电脑上的工作区编号';
COMMENT ON COLUMN conversation_assistant_workspaces.assigned_by_user_id IS '指定工作区的助理主人编号';

ALTER TABLE agent_runs ADD COLUMN execution_workspace_id uuid;

COMMENT ON COLUMN agent_runs.execution_workspace_id IS '执行工作区编号，派发时按会话中助理的工作区写入，为空表示不使用本机工具';

CREATE UNIQUE INDEX agent_runs_running_workspace_unique
    ON agent_runs (organization_id, execution_workspace_id)
    WHERE execution_workspace_id IS NOT NULL AND status = 'running';
