-- +goose Up
ALTER TABLE messages ADD COLUMN visibility text NOT NULL DEFAULT 'customer_visible';
COMMENT ON COLUMN messages.visibility IS '消息可见范围：customer_visible 对客户可见，internal_only 仅企业成员可见';

-- +goose Down
ALTER TABLE messages DROP COLUMN visibility;
