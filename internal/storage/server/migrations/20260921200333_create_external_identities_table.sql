-- +goose Up
-- 创建外部身份绑定表。
CREATE TABLE external_identities (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    organization_id uuid NOT NULL,
    user_id         uuid NOT NULL,
    issuer          text NOT NULL,
    subject         text NOT NULL
);

CREATE UNIQUE INDEX external_identities_organization_issuer_subject_unique
    ON external_identities (organization_id, issuer, subject);

CREATE UNIQUE INDEX external_identities_user_unique
    ON external_identities (user_id);

COMMENT ON TABLE external_identities IS '企业用户与官方身份服务账号的绑定';
COMMENT ON COLUMN external_identities.id IS '绑定编号';
COMMENT ON COLUMN external_identities.created_at IS '创建时间';
COMMENT ON COLUMN external_identities.updated_at IS '更新时间';
COMMENT ON COLUMN external_identities.organization_id IS '所属企业编号';
COMMENT ON COLUMN external_identities.user_id IS '企业用户编号';
COMMENT ON COLUMN external_identities.issuer IS '可信身份服务标识';
COMMENT ON COLUMN external_identities.subject IS '身份服务中的稳定账号标识';

-- +goose Down
DROP TABLE external_identities;
