-- +goose Up
-- 联系人增加工作区内编号。
ALTER TABLE contacts ADD COLUMN number bigint NOT NULL;

CREATE UNIQUE INDEX contacts_organization_number_unique
    ON contacts (organization_id, number);

COMMENT ON COLUMN contacts.number IS '工作区内联系人编号，从 1 开始按创建顺序递增';

-- +goose Down
DROP INDEX contacts_organization_number_unique;
ALTER TABLE contacts DROP COLUMN number;
