-- +goose Up
-- 客服处理周期增加小结与交接摘要，企业客服设置增加判断模型、小结模型与小结语言。
ALTER TABLE service_sessions
    ADD COLUMN summary_status                text,
    ADD COLUMN summary                       text,
    ADD COLUMN resolved                      boolean,
    ADD COLUMN summary_edited_by_identity_id uuid,
    ADD COLUMN summary_edited_at             timestamptz,
    ADD COLUMN handoff_message_id            uuid,
    ADD COLUMN handoff_summary               jsonb;

ALTER TABLE customer_service_settings
    ADD COLUMN decision_provider_id      uuid,
    ADD COLUMN decision_model_identifier text,
    ADD COLUMN summary_provider_id       uuid,
    ADD COLUMN summary_model_identifier  text,
    ADD COLUMN summary_locale            text NOT NULL DEFAULT 'zh-CN';

COMMENT ON COLUMN service_sessions.summary_status IS '小结状态：pending 等待生成、ready 已生成、no_request 无实质诉求、failed 生成失败；周期未关闭或不生成小结时为空';
COMMENT ON COLUMN service_sessions.summary IS '小结正文，由小结模型生成或客服填写；未生成时为空';
COMMENT ON COLUMN service_sessions.resolved IS '小结标注的是否解决，由判断模型标注或客服填写；无法判断时为空';
COMMENT ON COLUMN service_sessions.summary_edited_by_identity_id IS '最后修改小结的成员企业身份编号；有值时小结、咨询分类与是否解决保持客服填写的结果';
COMMENT ON COLUMN service_sessions.summary_edited_at IS '客服最后修改小结的时间';
COMMENT ON COLUMN service_sessions.handoff_message_id IS '最近一次转人工系统事件的消息编号，交接摘要归属于该事件';
COMMENT ON COLUMN service_sessions.handoff_summary IS '最近一次转人工的交接摘要：request 客户诉求、progress AI 已完成的处理、blocker 需要人工处理的卡点；未生成时为空';
COMMENT ON COLUMN service_sessions.category_id IS '咨询分类编号，由 AI 转人工或周期小结写入，客服修改小结时可调整';
COMMENT ON COLUMN customer_service_settings.decision_provider_id IS '标注周期实质诉求、咨询分类与是否解决的判断模型供应商编号；为空时不标注';
COMMENT ON COLUMN customer_service_settings.decision_model_identifier IS '判断模型标识';
COMMENT ON COLUMN customer_service_settings.summary_provider_id IS '生成周期小结与交接摘要的对话模型供应商编号；为空时不生成正文';
COMMENT ON COLUMN customer_service_settings.summary_model_identifier IS '小结模型标识';
COMMENT ON COLUMN customer_service_settings.summary_locale IS '小结与交接摘要使用的语言';

-- +goose Down
COMMENT ON COLUMN service_sessions.category_id IS '咨询分类编号，由 AI 转人工时写入';

ALTER TABLE customer_service_settings
    DROP COLUMN summary_locale,
    DROP COLUMN summary_model_identifier,
    DROP COLUMN summary_provider_id,
    DROP COLUMN decision_model_identifier,
    DROP COLUMN decision_provider_id;

ALTER TABLE service_sessions
    DROP COLUMN handoff_summary,
    DROP COLUMN handoff_message_id,
    DROP COLUMN summary_edited_at,
    DROP COLUMN summary_edited_by_identity_id,
    DROP COLUMN resolved,
    DROP COLUMN summary,
    DROP COLUMN summary_status;
