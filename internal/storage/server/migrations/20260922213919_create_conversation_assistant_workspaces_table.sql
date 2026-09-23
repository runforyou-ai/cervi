-- +goose Up
-- 创建会话助理工作区表。
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

-- +goose Down
DROP TABLE conversation_assistant_workspaces;
