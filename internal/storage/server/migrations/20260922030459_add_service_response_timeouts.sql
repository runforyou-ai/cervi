-- +goose Up
-- 企业客服设置增加未响应提醒、未响应回收与队列等待提醒时长，客服处理周期增加本轮等待的提醒时间。
ALTER TABLE customer_service_settings
    ADD COLUMN response_reminder_minutes integer NOT NULL DEFAULT 5,
    ADD COLUMN response_reclaim_minutes integer NOT NULL DEFAULT 15,
    ADD COLUMN queue_reminder_minutes integer NOT NULL DEFAULT 5;

ALTER TABLE service_sessions
    ADD COLUMN reminded_at timestamptz;

COMMENT ON COLUMN customer_service_settings.response_reminder_minutes IS '负责人超过该分钟数未回复客户时提醒负责人';
COMMENT ON COLUMN customer_service_settings.response_reclaim_minutes IS '负责人超过该分钟数未回复客户时退回队列并重新分配，大于未响应提醒时长';
COMMENT ON COLUMN customer_service_settings.queue_reminder_minutes IS '周期在队列中等待超过该分钟数时提醒对应客服';
COMMENT ON COLUMN service_sessions.reminded_at IS '本轮等待已发出超时提醒的时间；负责人或等待起点变化时清空，队列中的周期换队列时同样清空';

-- +goose Down
ALTER TABLE service_sessions
    DROP COLUMN reminded_at;

ALTER TABLE customer_service_settings
    DROP COLUMN queue_reminder_minutes,
    DROP COLUMN response_reclaim_minutes,
    DROP COLUMN response_reminder_minutes;
