-- +goose Up
-- 创建客户会话客服处理周期表。
CREATE TABLE service_sessions (
    id                           uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at                   timestamptz NOT NULL DEFAULT now(),
    updated_at                   timestamptz NOT NULL DEFAULT now(),
    organization_id              uuid NOT NULL,
    conversation_id              uuid NOT NULL,
    contact_channel_identity_id  uuid NOT NULL,
    sequence                     bigint NOT NULL,
    status                       text NOT NULL DEFAULT 'waiting',
    team_id                      uuid,
    assignee_identity_id         uuid,
    opening_message_id           uuid NOT NULL,
    last_message_id              uuid NOT NULL,
    last_message_at              timestamptz NOT NULL,
    assigned_at                  timestamptz,
    first_response_at            timestamptz,
    status_changed_at            timestamptz NOT NULL DEFAULT now(),
    closed_at                    timestamptz,
    closed_by_identity_id        uuid
);

COMMENT ON TABLE service_sessions IS '客户会话客服处理周期';
COMMENT ON COLUMN service_sessions.id IS '客服处理周期编号';
COMMENT ON COLUMN service_sessions.created_at IS '创建时间';
COMMENT ON COLUMN service_sessions.updated_at IS '更新时间';
COMMENT ON COLUMN service_sessions.organization_id IS '所属企业编号';
COMMENT ON COLUMN service_sessions.conversation_id IS '客户会话编号';
COMMENT ON COLUMN service_sessions.contact_channel_identity_id IS '联系人渠道身份编号';
COMMENT ON COLUMN service_sessions.sequence IS '会话内处理周期序号';
COMMENT ON COLUMN service_sessions.status IS '客服处理状态：waiting、active、pending、closed';
COMMENT ON COLUMN service_sessions.team_id IS '负责团队编号';
COMMENT ON COLUMN service_sessions.assignee_identity_id IS '负责人企业身份编号';
COMMENT ON COLUMN service_sessions.opening_message_id IS '处理周期首条消息编号';
COMMENT ON COLUMN service_sessions.last_message_id IS '处理周期最后消息编号';
COMMENT ON COLUMN service_sessions.last_message_at IS '处理周期最后消息发生时间';
COMMENT ON COLUMN service_sessions.assigned_at IS '首次分配时间';
COMMENT ON COLUMN service_sessions.first_response_at IS '首次客服响应时间';
COMMENT ON COLUMN service_sessions.status_changed_at IS '处理状态最后变更时间';
COMMENT ON COLUMN service_sessions.closed_at IS '关闭时间';
COMMENT ON COLUMN service_sessions.closed_by_identity_id IS '关闭人企业身份编号';

CREATE UNIQUE INDEX service_sessions_org_conversation_sequence_unique
    ON service_sessions (
        organization_id,
        conversation_id,
        sequence
    );

COMMENT ON INDEX service_sessions_org_conversation_sequence_unique
    IS '企业客户会话处理周期序号唯一索引';

CREATE UNIQUE INDEX service_sessions_org_opening_message_unique
    ON service_sessions (organization_id, opening_message_id);

COMMENT ON INDEX service_sessions_org_opening_message_unique
    IS '企业客服处理周期首条消息唯一索引';

CREATE UNIQUE INDEX service_sessions_org_conversation_open_unique
    ON service_sessions (organization_id, conversation_id)
    WHERE status IN ('waiting', 'active', 'pending');

COMMENT ON INDEX service_sessions_org_conversation_open_unique
    IS '企业客户会话未结束处理周期唯一索引';

CREATE UNIQUE INDEX service_sessions_org_channel_identity_open_unique
    ON service_sessions (organization_id, contact_channel_identity_id)
    WHERE status IN ('waiting', 'active', 'pending');

COMMENT ON INDEX service_sessions_org_channel_identity_open_unique
    IS '企业渠道身份未结束处理周期唯一索引';

CREATE INDEX service_sessions_org_conversation_last_message_index
    ON service_sessions (
        organization_id,
        conversation_id,
        last_message_at DESC,
        id DESC
    );

COMMENT ON INDEX service_sessions_org_conversation_last_message_index
    IS '企业客户会话处理周期最后消息索引';

CREATE INDEX service_sessions_org_channel_identity_last_message_index
    ON service_sessions (
        organization_id,
        contact_channel_identity_id,
        last_message_at DESC,
        id DESC
    );

COMMENT ON INDEX service_sessions_org_channel_identity_last_message_index
    IS '企业渠道身份处理周期最后消息索引';

-- +goose Down
DROP TABLE service_sessions;
