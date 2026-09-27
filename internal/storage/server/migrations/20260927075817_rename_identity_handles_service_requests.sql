-- +goose Up
-- 真人成员的接待开关改为处理服务请求，适用于客户、员工与伙伴的服务会话。
ALTER TABLE organization_identities RENAME COLUMN handles_customers TO handles_service_requests;

COMMENT ON COLUMN organization_identities.handles_service_requests IS '真人成员是否处理服务请求';

-- +goose Down
ALTER TABLE organization_identities RENAME COLUMN handles_service_requests TO handles_customers;

COMMENT ON COLUMN organization_identities.handles_customers IS '真人成员是否接待客户';
