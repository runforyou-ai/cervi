-- +goose Up
-- 企业增加生命周期状态。
ALTER TABLE organizations
    ADD COLUMN lifecycle_status text NOT NULL DEFAULT 'active';

COMMENT ON COLUMN organizations.lifecycle_status IS '企业生命周期状态：active、suspended、deleting、deleted';

-- +goose Down
ALTER TABLE organizations
    DROP COLUMN lifecycle_status;
