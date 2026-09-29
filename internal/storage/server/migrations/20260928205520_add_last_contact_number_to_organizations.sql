-- +goose Up
-- 工作区增加联系人编号计数。
ALTER TABLE organizations ADD COLUMN last_contact_number bigint NOT NULL DEFAULT 0;

COMMENT ON COLUMN organizations.last_contact_number IS '工作区最近分配的联系人编号';

-- +goose Down
ALTER TABLE organizations DROP COLUMN last_contact_number;
