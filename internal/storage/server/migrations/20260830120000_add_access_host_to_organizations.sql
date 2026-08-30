-- +goose Up
ALTER TABLE organizations
    ADD COLUMN access_host text NOT NULL;

COMMENT ON COLUMN organizations.access_host IS '企业规范化访问地址，包含非默认端口';

CREATE UNIQUE INDEX organizations_access_host_unique
    ON organizations (access_host);

-- +goose Down
DROP INDEX organizations_access_host_unique;

ALTER TABLE organizations
    DROP COLUMN access_host;
