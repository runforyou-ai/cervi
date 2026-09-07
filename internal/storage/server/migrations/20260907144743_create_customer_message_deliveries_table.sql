-- +goose Up
CREATE TABLE customer_message_deliveries (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    message_id uuid NOT NULL,
    channel_id uuid NOT NULL,
    contact_channel_identity_id uuid NOT NULL,
    bot_id bigint NOT NULL,
    position bigint NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    attempt integer NOT NULL DEFAULT 0,
    provider_message_id bigint,
    lease_worker uuid,
    lease_expires_at timestamptz,
    available_at timestamptz NOT NULL DEFAULT now(),
    uncertain_until timestamptz,
    last_error text NOT NULL DEFAULT '',
    sent_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE customer_message_deliveries IS '客户消息外部投递';
COMMENT ON COLUMN customer_message_deliveries.id IS '投递编号';
COMMENT ON COLUMN customer_message_deliveries.organization_id IS '企业编号';
COMMENT ON COLUMN customer_message_deliveries.conversation_id IS '会话编号';
COMMENT ON COLUMN customer_message_deliveries.message_id IS '本地消息编号';
COMMENT ON COLUMN customer_message_deliveries.channel_id IS '渠道编号';
COMMENT ON COLUMN customer_message_deliveries.contact_channel_identity_id IS '接收渠道身份';
COMMENT ON COLUMN customer_message_deliveries.bot_id IS '发送机器人编号';
COMMENT ON COLUMN customer_message_deliveries.position IS '身份内发送顺序';
COMMENT ON COLUMN customer_message_deliveries.status IS '投递状态';
COMMENT ON COLUMN customer_message_deliveries.attempt IS '发送尝试次数';
COMMENT ON COLUMN customer_message_deliveries.provider_message_id IS '平台消息编号';
COMMENT ON COLUMN customer_message_deliveries.lease_worker IS '发送认领标识';
COMMENT ON COLUMN customer_message_deliveries.lease_expires_at IS '认领过期时间';
COMMENT ON COLUMN customer_message_deliveries.available_at IS '允许发送时间';
COMMENT ON COLUMN customer_message_deliveries.uncertain_until IS '结果确认期限';
COMMENT ON COLUMN customer_message_deliveries.last_error IS '投递错误码';
COMMENT ON COLUMN customer_message_deliveries.sent_at IS '发送确认时间';
COMMENT ON COLUMN customer_message_deliveries.created_at IS '创建时间';
COMMENT ON COLUMN customer_message_deliveries.updated_at IS '更新时间';

CREATE UNIQUE INDEX customer_deliveries_message_unique ON customer_message_deliveries (organization_id, message_id);
CREATE UNIQUE INDEX customer_deliveries_position_unique ON customer_message_deliveries (channel_id, contact_channel_identity_id, position);
CREATE UNIQUE INDEX customer_deliveries_provider_unique ON customer_message_deliveries (channel_id, bot_id, contact_channel_identity_id, provider_message_id) WHERE provider_message_id IS NOT NULL;
CREATE UNIQUE INDEX customer_deliveries_active_identity_unique ON customer_message_deliveries (channel_id, contact_channel_identity_id) WHERE status IN ('sending', 'uncertain');
CREATE UNIQUE INDEX customer_deliveries_sending_channel_unique ON customer_message_deliveries (channel_id) WHERE status = 'sending';

-- +goose Down
DROP TABLE customer_message_deliveries;
