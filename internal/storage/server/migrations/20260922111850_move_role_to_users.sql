-- +goose Up
-- 企业角色改由用户账号持有。
ALTER TABLE users ADD COLUMN role_id uuid;

UPDATE users AS u
SET role_id = oi.role_id
FROM organization_identities AS oi
WHERE oi.id = u.identity_id AND oi.organization_id = u.organization_id;

ALTER TABLE users ALTER COLUMN role_id SET NOT NULL;

ALTER TABLE organization_identities DROP COLUMN role_id;

COMMENT ON COLUMN users.role_id IS '企业角色编号';

-- +goose Down
ALTER TABLE organization_identities ADD COLUMN role_id uuid;

UPDATE organization_identities AS oi
SET role_id = u.role_id
FROM users AS u
WHERE u.identity_id = oi.id AND u.organization_id = oi.organization_id;

UPDATE organization_identities AS oi
SET role_id = r.id
FROM roles AS r
WHERE oi.role_id IS NULL AND r.organization_id = oi.organization_id AND r.kind = 'member';

ALTER TABLE organization_identities ALTER COLUMN role_id SET NOT NULL;

COMMENT ON COLUMN organization_identities.role_id IS '企业角色编号';

ALTER TABLE users DROP COLUMN role_id;
