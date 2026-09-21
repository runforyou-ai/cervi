-- +goose Up
-- 创建会话设备绑定表。
CREATE TABLE conversation_device_bindings (
    id                      uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    organization_id         uuid NOT NULL,
    conversation_id         uuid NOT NULL,
    device_id               uuid NOT NULL,
    workspace_id            uuid NOT NULL,
    peer_trigger_capability text NOT NULL,
    bound_by_user_id        uuid NOT NULL
);

CREATE UNIQUE INDEX conversation_device_bindings_conversation_unique
    ON conversation_device_bindings (organization_id, conversation_id);

COMMENT ON TABLE conversation_device_bindings IS '会话绑定的执行设备与工作区，绑定期间该会话的 Agent 运行在设备上执行';
COMMENT ON COLUMN conversation_device_bindings.id IS '绑定编号';
COMMENT ON COLUMN conversation_device_bindings.created_at IS '绑定时间';
COMMENT ON COLUMN conversation_device_bindings.updated_at IS '更新时间';
COMMENT ON COLUMN conversation_device_bindings.organization_id IS '所属企业编号';
COMMENT ON COLUMN conversation_device_bindings.conversation_id IS '绑定的会话编号';
COMMENT ON COLUMN conversation_device_bindings.device_id IS '执行设备编号';
COMMENT ON COLUMN conversation_device_bindings.workspace_id IS '执行工作区编号';
COMMENT ON COLUMN conversation_device_bindings.peer_trigger_capability IS '设备主人以外的成员触发时的能力等级：off、read_only、write_with_approval、same_as_owner';
COMMENT ON COLUMN conversation_device_bindings.bound_by_user_id IS '执行绑定的设备主人编号';

-- +goose Down
DROP TABLE conversation_device_bindings;
