-- +goose Up
-- 客服处理周期增加咨询分类，转人工原因区分业务原因与系统原因。
ALTER TABLE service_sessions
    ADD COLUMN category_id uuid;

COMMENT ON COLUMN service_sessions.category_id IS '咨询分类编号，由 AI 转人工时写入';
COMMENT ON COLUMN agent_runs.outcome_reason IS '转交人工的原因；业务原因由 AI 给出：knowledge_gap 知识不足、customer_requested 客户要求真人、needs_human_judgment 需要人工判断、complaint 投诉；系统原因由 Runtime 给出：insufficient_evidence、budget_exhausted、invalid_output、runtime_failed、timeout、agent_unavailable';

-- +goose Down
COMMENT ON COLUMN agent_runs.outcome_reason IS '转交人工的原因：model_requested、insufficient_evidence、budget_exhausted、invalid_output、runtime_failed、timeout';

ALTER TABLE service_sessions
    DROP COLUMN category_id;
