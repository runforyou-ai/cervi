-- +goose Up
-- 创建团队成员关系表，关联关系由 Action 维护。
CREATE TABLE team_members (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    organization_id     uuid NOT NULL,
    team_id             uuid NOT NULL,
    member_id           uuid NOT NULL,
    created_by_user_id  uuid NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE team_members IS '团队成员关系';
COMMENT ON COLUMN team_members.id IS '关系编号';
COMMENT ON COLUMN team_members.organization_id IS '所属企业编号';
COMMENT ON COLUMN team_members.team_id IS '所属团队编号';
COMMENT ON COLUMN team_members.member_id IS '成员编号';
COMMENT ON COLUMN team_members.created_by_user_id IS '创建用户编号';
COMMENT ON COLUMN team_members.created_at IS '创建时间';

CREATE UNIQUE INDEX team_members_organization_team_member_unique
    ON team_members (organization_id, team_id, member_id);

CREATE INDEX team_members_organization_member_index
    ON team_members (organization_id, member_id, team_id);

-- +goose Down
DROP TABLE team_members;
