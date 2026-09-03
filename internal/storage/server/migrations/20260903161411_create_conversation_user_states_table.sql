-- +goose Up
-- 创建用户会话状态表。
CREATE TABLE conversation_user_states (
    id                    uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    organization_id       uuid NOT NULL,
    conversation_id       uuid NOT NULL,
    user_id               uuid NOT NULL,
    last_read_message_id  uuid NOT NULL,
    last_read_at          timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX conversation_user_states_org_conversation_user_unique
    ON conversation_user_states (organization_id, conversation_id, user_id);

CREATE INDEX conversation_user_states_organization_user_index
    ON conversation_user_states (organization_id, user_id, conversation_id);

COMMENT ON TABLE conversation_user_states IS '用户会话个人状态';
COMMENT ON COLUMN conversation_user_states.id IS '用户会话状态编号';
COMMENT ON COLUMN conversation_user_states.created_at IS '创建时间';
COMMENT ON COLUMN conversation_user_states.updated_at IS '更新时间';
COMMENT ON COLUMN conversation_user_states.organization_id IS '所属企业编号';
COMMENT ON COLUMN conversation_user_states.conversation_id IS '会话编号';
COMMENT ON COLUMN conversation_user_states.user_id IS '用户账号编号';
COMMENT ON COLUMN conversation_user_states.last_read_message_id IS '最后已读消息编号';
COMMENT ON COLUMN conversation_user_states.last_read_at IS '最后标记已读时间';
COMMENT ON INDEX conversation_user_states_org_conversation_user_unique
    IS '企业会话用户状态唯一索引';
COMMENT ON INDEX conversation_user_states_organization_user_index
    IS '企业用户会话状态查询索引';

-- +goose Down
DROP TABLE conversation_user_states;
