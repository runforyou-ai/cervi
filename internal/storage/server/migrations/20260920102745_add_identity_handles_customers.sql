-- +goose Up
-- 企业身份增加接待客户开关。
ALTER TABLE organization_identities ADD COLUMN handles_customers boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN organization_identities.handles_customers IS '是否接待客户';

-- +goose Down
ALTER TABLE organization_identities DROP COLUMN handles_customers;
