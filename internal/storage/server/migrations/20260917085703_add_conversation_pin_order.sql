-- +goose Up
ALTER TABLE conversation_user_states
    ADD COLUMN pin_rank bigint;
COMMENT ON COLUMN conversation_user_states.pin_rank IS '个人置顶顺序值，按升序排列，未置顶时为空';

-- 约束延迟到提交时检查，顺序值间隔耗尽后可在同一事务内整区重编号。
ALTER TABLE conversation_user_states
    ADD CONSTRAINT conversation_user_states_user_pin_rank_unique
    UNIQUE (organization_id, user_id, pin_rank) DEFERRABLE INITIALLY DEFERRED;
COMMENT ON CONSTRAINT conversation_user_states_user_pin_rank_unique ON conversation_user_states
    IS '企业用户置顶顺序值唯一约束';

ALTER TABLE users
    ADD COLUMN pin_order_version bigint NOT NULL DEFAULT 0;
COMMENT ON COLUMN users.pin_order_version IS '个人置顶顺序版本，置顶、取消置顶与调整顺序时推进';

-- +goose Down
ALTER TABLE users
    DROP COLUMN pin_order_version;
ALTER TABLE conversation_user_states
    DROP CONSTRAINT conversation_user_states_user_pin_rank_unique;
ALTER TABLE conversation_user_states
    DROP COLUMN pin_rank;
