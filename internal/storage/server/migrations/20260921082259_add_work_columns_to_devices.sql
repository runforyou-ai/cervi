-- +goose Up
-- 为设备增加工作水位与最近在线时间。
ALTER TABLE devices
    ADD COLUMN work_seq     bigint NOT NULL DEFAULT 0,
    ADD COLUMN last_seen_at timestamptz;

COMMENT ON COLUMN devices.work_seq IS '设备工作水位，派发或停止该设备执行的运行时递增';
COMMENT ON COLUMN devices.last_seen_at IS '设备最近一次以设备身份访问服务端的时间';

-- +goose Down
ALTER TABLE devices
    DROP COLUMN last_seen_at,
    DROP COLUMN work_seq;
