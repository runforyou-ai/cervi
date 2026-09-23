-- +goose Up
-- 客服处理周期增加结束方式与 AI 请求确认解决的时间，企业客服设置增加 AI 超时跟进与超时关单时长。
ALTER TABLE service_sessions
    ADD COLUMN close_reason text,
    ADD COLUMN resolution_requested_at timestamptz;

ALTER TABLE customer_service_settings
    ADD COLUMN ai_follow_up_minutes integer NOT NULL DEFAULT 10,
    ADD COLUMN ai_close_minutes integer NOT NULL DEFAULT 30;

COMMENT ON COLUMN service_sessions.close_reason IS '结束方式：ai_resolved AI 解决、customer_unresponsive 客户失联、manual 人工关闭；未关闭时为空';
COMMENT ON COLUMN service_sessions.resolution_requested_at IS 'AI 负责人请求客户确认问题是否解决的时间，包括超时跟进；周期出现新的对客消息时清空，早于当前负责人接手时间时不计入超时关单';
COMMENT ON COLUMN customer_service_settings.ai_follow_up_minutes IS 'AI 负责的周期中客户超过该分钟数未回复 AI 时，AI 跟进一次并请客户确认问题是否解决';
COMMENT ON COLUMN customer_service_settings.ai_close_minutes IS 'AI 跟进或请求确认后客户超过该分钟数仍未回复时关闭周期，结束方式记为客户失联';
COMMENT ON COLUMN agent_inputs.kind IS '输入入口：mention、handoff、agent_direct、customer_auto、copilot、follow_up';
COMMENT ON COLUMN agent_runs.outcome IS '运行结果类型：reply 回答、ask_customer 追问客户、handoff 转交人工、resolve 确认解决并关闭周期；未结束或被取消时为空';

-- +goose Down
COMMENT ON COLUMN agent_runs.outcome IS '运行结果类型：reply 回答、ask_customer 追问客户、handoff 转交人工；未结束或被取消时为空';
COMMENT ON COLUMN agent_inputs.kind IS '输入入口：mention、handoff、agent_direct、customer_auto、copilot';

ALTER TABLE customer_service_settings
    DROP COLUMN ai_close_minutes,
    DROP COLUMN ai_follow_up_minutes;

ALTER TABLE service_sessions
    DROP COLUMN resolution_requested_at,
    DROP COLUMN close_reason;
