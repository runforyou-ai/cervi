-- +goose Up
CREATE TABLE customer_channel_send_gates (
    channel_id uuid PRIMARY KEY,
    organization_id uuid NOT NULL,
    flood_wait_until timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE customer_channel_send_gates IS '客户渠道发送等待状态';
COMMENT ON COLUMN customer_channel_send_gates.channel_id IS '渠道编号';
COMMENT ON COLUMN customer_channel_send_gates.organization_id IS '企业编号';
COMMENT ON COLUMN customer_channel_send_gates.flood_wait_until IS '平台限流等待截止时间';
COMMENT ON COLUMN customer_channel_send_gates.updated_at IS '更新时间';

-- +goose Down
DROP TABLE customer_channel_send_gates;
