-- +goose Up
ALTER TABLE organizations
    ADD COLUMN access_host text NOT NULL DEFAULT '';

ALTER TABLE organizations
    ALTER COLUMN access_host DROP DEFAULT;

COMMENT ON COLUMN organizations.access_host IS '企业规范化访问地址，包含非默认端口；空字符串仅标记升级前的唯一企业';

CREATE UNIQUE INDEX organizations_access_host_unique
    ON organizations (access_host);

-- +goose Down
DROP INDEX organizations_access_host_unique;

ALTER TABLE organizations
    DROP COLUMN access_host;
