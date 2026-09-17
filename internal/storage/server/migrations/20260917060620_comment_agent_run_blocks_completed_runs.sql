-- +goose Up
-- 过程内容块在运行失败或被取消时同样保留。
COMMENT ON TABLE agent_run_blocks IS '已完成 Agent 运行的有序中间内容';

-- +goose Down
COMMENT ON TABLE agent_run_blocks IS '成功 Agent 运行的有序中间内容';
