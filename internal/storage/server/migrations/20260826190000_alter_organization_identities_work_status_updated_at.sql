-- +goose Up
-- 将工作状态更新时间改为 timestamptz 并使用 now() 默认值，与全库其余时间列保持一致。
ALTER TABLE organization_identities
    ALTER COLUMN work_status_updated_at TYPE timestamptz USING work_status_updated_at AT TIME ZONE 'UTC',
    ALTER COLUMN work_status_updated_at SET NOT NULL,
    ALTER COLUMN work_status_updated_at SET DEFAULT now();

COMMENT ON COLUMN organization_identities.work_status_updated_at IS '工作状态更新时间';

-- +goose Down
-- 恢复为不带时区的 timestamp 和 CURRENT_TIMESTAMP 默认值。
ALTER TABLE organization_identities
    ALTER COLUMN work_status_updated_at TYPE timestamp without time zone USING work_status_updated_at AT TIME ZONE 'UTC',
    ALTER COLUMN work_status_updated_at SET NOT NULL,
    ALTER COLUMN work_status_updated_at SET DEFAULT CURRENT_TIMESTAMP;
