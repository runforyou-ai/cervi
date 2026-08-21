-- +goose Up
-- 为企业成员保存主动设置的工作状态及其更新时间。
ALTER TABLE users
    ADD COLUMN work_status text NOT NULL DEFAULT 'working',
    ADD COLUMN work_status_updated_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP;

COMMENT ON COLUMN users.work_status IS '用户主动设置的工作状态';
COMMENT ON COLUMN users.work_status_updated_at IS '工作状态更新时间';

-- +goose Down
ALTER TABLE users
    DROP COLUMN work_status_updated_at,
    DROP COLUMN work_status;
