-- +goose Up
-- 客户会话关系改为渠道会话关系，服务周期与 Copilot 线程改挂服务会话，消息可见范围改为共享与内部。
ALTER TABLE customer_conversations RENAME TO channel_conversations;

ALTER TABLE channel_conversations DROP COLUMN current_service_session_id;
ALTER TABLE channel_conversations RENAME COLUMN customer_read_seq TO contact_read_seq;
ALTER TABLE channel_conversations RENAME COLUMN customer_notified_seq TO contact_notified_seq;
ALTER TABLE channel_conversations RENAME COLUMN customer_notify_due_at TO contact_notify_due_at;

COMMENT ON TABLE channel_conversations IS '渠道会话与联系人渠道身份的关系';
COMMENT ON COLUMN channel_conversations.conversation_id IS '渠道会话编号';
COMMENT ON COLUMN channel_conversations.reply_language IS '客服锁定的对联系人回复语言，BCP 47 语言标签；为空时按联系人最近消息的语言回复';
COMMENT ON COLUMN channel_conversations.contact_read_seq IS '联系人在网站 Messenger 中已读到的最大消息序号，未读过时为 0';
COMMENT ON COLUMN channel_conversations.contact_notified_seq IS '已通过邮件通知联系人的最大消息序号，未通知过时为 0';
COMMENT ON COLUMN channel_conversations.contact_notify_due_at IS '检查未读真人回复并发送邮件通知的时间，由第一条未通知的真人回复起算；没有待通知回复时为空';

DROP INDEX service_sessions_organization_conversation_sequence_unique;
DROP INDEX service_sessions_organization_conversation_open_unique;

ALTER TABLE service_sessions
    DROP COLUMN contact_channel_identity_id,
    ADD COLUMN service_conversation_id uuid NOT NULL;

CREATE UNIQUE INDEX service_sessions_organization_service_conversation_sequence_unique
    ON service_sessions (organization_id, service_conversation_id, sequence);

CREATE UNIQUE INDEX service_sessions_organization_service_conversation_open_unique
    ON service_sessions (organization_id, service_conversation_id)
    WHERE status = 'open';

COMMENT ON TABLE service_sessions IS '服务周期';
COMMENT ON COLUMN service_sessions.id IS '服务周期编号';
COMMENT ON COLUMN service_sessions.conversation_id IS '承载服务会话的会话编号';
COMMENT ON COLUMN service_sessions.service_conversation_id IS '所属服务会话编号';
COMMENT ON COLUMN service_sessions.sequence IS '服务会话内的周期序号';
COMMENT ON INDEX service_sessions_organization_service_conversation_sequence_unique
    IS '企业服务会话周期序号唯一索引';
COMMENT ON INDEX service_sessions_organization_service_conversation_open_unique
    IS '企业服务会话未结束周期唯一索引';

ALTER TABLE customer_copilot_threads RENAME TO service_copilot_threads;
ALTER TABLE service_copilot_threads RENAME COLUMN customer_conversation_id TO served_conversation_id;

COMMENT ON TABLE service_copilot_threads IS '服务会话 Copilot 线程归属';
COMMENT ON COLUMN service_copilot_threads.served_conversation_id IS '承载所属服务会话的会话编号';

ALTER TABLE messages ALTER COLUMN visibility SET DEFAULT 'shared';

COMMENT ON COLUMN messages.service_session_id IS '所属服务周期编号';
COMMENT ON COLUMN messages.visibility IS '消息可见范围：shared 会话各方可见，internal 仅服务会话的处理方可见';
COMMENT ON COLUMN conversations.type IS '会话类型：direct、group、agent、channel、copilot';

-- +goose Down
COMMENT ON COLUMN conversations.type IS '会话类型：direct、group、agent、customer、copilot';
COMMENT ON COLUMN messages.visibility IS '消息可见范围：customer_visible 对客户可见，internal_only 仅企业成员可见';
COMMENT ON COLUMN messages.service_session_id IS '所属客服处理周期编号';

ALTER TABLE messages ALTER COLUMN visibility SET DEFAULT 'customer_visible';

ALTER TABLE service_copilot_threads RENAME COLUMN served_conversation_id TO customer_conversation_id;
ALTER TABLE service_copilot_threads RENAME TO customer_copilot_threads;

COMMENT ON TABLE customer_copilot_threads IS '客户会话 Copilot 线程归属';
COMMENT ON COLUMN customer_copilot_threads.customer_conversation_id IS '所属客户会话编号';

DROP INDEX service_sessions_organization_service_conversation_open_unique;
DROP INDEX service_sessions_organization_service_conversation_sequence_unique;

ALTER TABLE service_sessions
    DROP COLUMN service_conversation_id,
    ADD COLUMN contact_channel_identity_id uuid NOT NULL;

CREATE UNIQUE INDEX service_sessions_organization_conversation_sequence_unique
    ON service_sessions (organization_id, conversation_id, sequence);

CREATE UNIQUE INDEX service_sessions_organization_conversation_open_unique
    ON service_sessions (organization_id, conversation_id)
    WHERE status = 'open';

COMMENT ON TABLE service_sessions IS '客户会话客服处理周期';
COMMENT ON COLUMN service_sessions.id IS '客服处理周期编号';
COMMENT ON COLUMN service_sessions.conversation_id IS '客户会话编号';
COMMENT ON COLUMN service_sessions.contact_channel_identity_id IS '联系人渠道身份编号';
COMMENT ON COLUMN service_sessions.sequence IS '会话内处理周期序号';

ALTER TABLE channel_conversations RENAME COLUMN contact_notify_due_at TO customer_notify_due_at;
ALTER TABLE channel_conversations RENAME COLUMN contact_notified_seq TO customer_notified_seq;
ALTER TABLE channel_conversations RENAME COLUMN contact_read_seq TO customer_read_seq;
ALTER TABLE channel_conversations ADD COLUMN current_service_session_id uuid;

COMMENT ON COLUMN channel_conversations.current_service_session_id IS '当前客服处理周期编号';

ALTER TABLE channel_conversations RENAME TO customer_conversations;

COMMENT ON TABLE customer_conversations IS '客户会话渠道关系';
COMMENT ON COLUMN customer_conversations.conversation_id IS '客户会话编号';
