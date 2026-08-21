-- +goose Up
-- 创建团队成员关系表，关联关系由 Action 维护。
CREATE TABLE team_members (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    organization_id     uuid NOT NULL,
    team_id             uuid NOT NULL,
    identity_type       text NOT NULL,
    identity_id         uuid NOT NULL,
    created_by_user_id  uuid NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX team_members_organization_team_identity_unique
    ON team_members (organization_id, team_id, identity_type, identity_id);

CREATE INDEX team_members_organization_identity_index
    ON team_members (organization_id, identity_type, identity_id, team_id);

-- +goose Down
DROP TABLE team_members;
