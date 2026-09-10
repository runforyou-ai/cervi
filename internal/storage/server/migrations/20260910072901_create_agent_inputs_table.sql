-- +goose Up
-- 创建 Agent 持久输入表。
CREATE TABLE agent_inputs (
    id                 uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at         timestamptz NOT NULL DEFAULT now(),
    organization_id    uuid NOT NULL,
    lane_id            uuid NOT NULL,
    input_seq          bigint NOT NULL,
    kind               text NOT NULL,
    source_message_id  uuid NOT NULL,
    source_subject_id  uuid NOT NULL,
    source_ordinal     integer NOT NULL DEFAULT 0,
    depth              integer NOT NULL DEFAULT 0,
    agent_run_id       uuid
);

CREATE UNIQUE INDEX agent_inputs_lane_seq_unique
    ON agent_inputs (lane_id, input_seq);

COMMENT ON TABLE agent_inputs IS '等待 Agent 消费的持久输入';
COMMENT ON COLUMN agent_inputs.id IS '输入编号';
COMMENT ON COLUMN agent_inputs.created_at IS '创建时间';
COMMENT ON COLUMN agent_inputs.organization_id IS '所属企业编号';
COMMENT ON COLUMN agent_inputs.lane_id IS '所属输入队列编号';
COMMENT ON COLUMN agent_inputs.input_seq IS '队列内连续输入序号';
COMMENT ON COLUMN agent_inputs.kind IS '输入入口：agent_direct、customer_auto';
COMMENT ON COLUMN agent_inputs.source_message_id IS '产生本次输入的消息编号';
COMMENT ON COLUMN agent_inputs.source_subject_id IS '产生本次输入的聊天主体编号';
COMMENT ON COLUMN agent_inputs.source_ordinal IS '同一条来源消息内的目标位置';
COMMENT ON COLUMN agent_inputs.depth IS '接力深度，真人发起为 0';
COMMENT ON COLUMN agent_inputs.agent_run_id IS '实际消费本次输入的 Agent 运行编号';

-- +goose Down
DROP TABLE agent_inputs;
