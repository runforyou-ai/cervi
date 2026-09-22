-- +goose Up
-- 创建运营开通幂等记录表。
CREATE TABLE operator_provisionings (
    provisioning_id text PRIMARY KEY,
    created_at      timestamptz NOT NULL DEFAULT now(),
    request_digest  text NOT NULL,
    organization_id uuid NOT NULL,
    initial_user_id uuid NOT NULL
);

CREATE UNIQUE INDEX operator_provisionings_organization_unique
    ON operator_provisionings (organization_id);

COMMENT ON TABLE operator_provisionings IS '运营开通请求的幂等记录';
COMMENT ON COLUMN operator_provisionings.provisioning_id IS 'SaaS 持久化的开通标识';
COMMENT ON COLUMN operator_provisionings.created_at IS '创建时间';
COMMENT ON COLUMN operator_provisionings.request_digest IS '规范化开通请求的 SHA-256 摘要';
COMMENT ON COLUMN operator_provisionings.organization_id IS '创建的企业编号';
COMMENT ON COLUMN operator_provisionings.initial_user_id IS '创建的初始用户编号';

-- +goose Down
DROP TABLE operator_provisionings;
