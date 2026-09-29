-- +goose Up
-- 移除渠道级发送中唯一约束，同一渠道身份只有一条发送中的投递由 customer_deliveries_active_identity_unique 保证。
DROP INDEX customer_deliveries_sending_channel_unique;

-- +goose Down
-- 同一渠道多条发送中的投递只保留最早一条，其余转为结果未知，等待人工确认。
UPDATE customer_message_deliveries AS d
SET status = 'uncertain', uncertain_until = now(), lease_worker = NULL, lease_expires_at = NULL, last_error = 'unknown_result', updated_at = now()
WHERE d.status = 'sending'
    AND EXISTS (
        SELECT 1 FROM customer_message_deliveries AS earlier
        WHERE earlier.channel_id = d.channel_id AND earlier.status = 'sending' AND earlier.id < d.id
    );

CREATE UNIQUE INDEX customer_deliveries_sending_channel_unique
    ON customer_message_deliveries (channel_id) WHERE (status = 'sending'::text);
