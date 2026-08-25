-- +goose Up
-- 创建客户会话渠道关系表。
CREATE TABLE customer_conversations (
    conversation_id              uuid PRIMARY KEY,
    created_at                   timestamptz NOT NULL DEFAULT now(),
    updated_at                   timestamptz NOT NULL DEFAULT now(),
    organization_id              uuid NOT NULL,
    contact_channel_identity_id  uuid NOT NULL
);

COMMENT ON TABLE customer_conversations IS '客户会话渠道关系';
COMMENT ON COLUMN customer_conversations.conversation_id IS '客户会话编号';
COMMENT ON COLUMN customer_conversations.created_at IS '创建时间';
COMMENT ON COLUMN customer_conversations.updated_at IS '更新时间';
COMMENT ON COLUMN customer_conversations.organization_id IS '所属企业编号';
COMMENT ON COLUMN customer_conversations.contact_channel_identity_id IS '联系人渠道身份编号';

CREATE INDEX customer_conversations_org_channel_identity_created_index
    ON customer_conversations (
        organization_id,
        contact_channel_identity_id,
        created_at DESC,
        conversation_id DESC
    );

COMMENT ON INDEX customer_conversations_org_channel_identity_created_index
    IS '企业内渠道身份的客户会话创建时间索引';

-- +goose Down
DROP TABLE customer_conversations;
