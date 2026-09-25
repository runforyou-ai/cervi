-- +goose Up
-- 消息增加正文语言，客户会话增加锁定的回复语言，用户增加翻译语言，企业客服设置增加翻译模型。
ALTER TABLE messages
    ADD COLUMN language text;

ALTER TABLE customer_conversations
    ADD COLUMN reply_language text;

ALTER TABLE users
    ADD COLUMN translation_language text;

ALTER TABLE customer_service_settings
    ADD COLUMN translation_provider_id      uuid,
    ADD COLUMN translation_model_identifier text;

COMMENT ON COLUMN messages.language IS '正文语言，BCP 47 语言标签，无语言内容时为 und；尚未识别时为空';
COMMENT ON COLUMN customer_conversations.reply_language IS '客服锁定的对客回复语言，BCP 47 语言标签；为空时按客户最近消息的语言回复';
COMMENT ON COLUMN users.translation_language IS '客户消息译文与翻译发送使用的本人语言，BCP 47 语言标签；为空时使用界面语言';
COMMENT ON COLUMN customer_service_settings.translation_provider_id IS '翻译客户会话消息的对话模型供应商编号；为空时不提供翻译';
COMMENT ON COLUMN customer_service_settings.translation_model_identifier IS '翻译模型标识';

-- +goose Down
ALTER TABLE customer_service_settings
    DROP COLUMN translation_model_identifier,
    DROP COLUMN translation_provider_id;

ALTER TABLE users
    DROP COLUMN translation_language;

ALTER TABLE customer_conversations
    DROP COLUMN reply_language;

ALTER TABLE messages
    DROP COLUMN language;
