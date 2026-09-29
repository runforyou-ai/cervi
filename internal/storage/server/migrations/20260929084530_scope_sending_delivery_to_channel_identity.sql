-- +goose Up
-- 同一渠道身份同时只有一条发送中的投递，不同渠道身份并行发送。
DROP INDEX customer_deliveries_sending_channel_unique;

CREATE UNIQUE INDEX customer_deliveries_sending_identity_unique
    ON customer_message_deliveries (contact_channel_identity_id) WHERE (status = 'sending'::text);

-- +goose Down
DROP INDEX customer_deliveries_sending_identity_unique;

CREATE UNIQUE INDEX customer_deliveries_sending_channel_unique
    ON customer_message_deliveries (channel_id) WHERE (status = 'sending'::text);
