-- +goose Up
-- 把企业角色统一归属到真人和 AI 共用的企业身份。
ALTER TABLE organization_identities
    ADD COLUMN role_id uuid;

UPDATE organization_identities AS oi
SET role_id = u.role_id
FROM users AS u
WHERE oi.organization_id = u.organization_id
  AND oi.id = u.identity_id
  AND oi.type = 'user';

UPDATE organization_identities AS oi
SET role_id = r.id
FROM roles AS r
WHERE oi.organization_id = r.organization_id
  AND oi.type = 'agent'
  AND r.kind = 'member';

ALTER TABLE organization_identities
    ALTER COLUMN role_id SET NOT NULL;

COMMENT ON COLUMN organization_identities.role_id IS '企业角色编号';

CREATE INDEX organization_identities_organization_role_index
    ON organization_identities (organization_id, role_id);

DROP INDEX users_organization_role_index;

ALTER TABLE users
    DROP COLUMN role_id;

-- +goose Down
ALTER TABLE users
    ADD COLUMN role_id uuid;

UPDATE users AS u
SET role_id = oi.role_id
FROM organization_identities AS oi
WHERE u.organization_id = oi.organization_id
  AND u.identity_id = oi.id
  AND oi.type = 'user';

ALTER TABLE users
    ALTER COLUMN role_id SET NOT NULL;

COMMENT ON COLUMN users.role_id IS '企业角色编号';

CREATE INDEX users_organization_role_index
    ON users (organization_id, role_id);

DROP INDEX organization_identities_organization_role_index;

ALTER TABLE organization_identities
    DROP COLUMN role_id;
