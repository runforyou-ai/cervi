-- +goose Up
-- 客户会话增加客户已读位置、邮件通知位置与邮件通知检查时间。
ALTER TABLE customer_conversations
    ADD COLUMN customer_read_seq bigint NOT NULL DEFAULT 0,
    ADD COLUMN customer_notified_seq bigint NOT NULL DEFAULT 0,
    ADD COLUMN customer_notify_due_at timestamptz;

COMMENT ON COLUMN customer_conversations.customer_read_seq IS '客户在网站 Messenger 中已读到的最大消息序号，未读过时为 0';
COMMENT ON COLUMN customer_conversations.customer_notified_seq IS '已通过邮件通知客户的最大消息序号，未通知过时为 0';
COMMENT ON COLUMN customer_conversations.customer_notify_due_at IS '检查未读真人回复并发送邮件通知的时间，由第一条未通知的真人回复起算；没有待通知回复时为空';

-- +goose Down
ALTER TABLE customer_conversations
    DROP COLUMN customer_notify_due_at,
    DROP COLUMN customer_notified_seq,
    DROP COLUMN customer_read_seq;
