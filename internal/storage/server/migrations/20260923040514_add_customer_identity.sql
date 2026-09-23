-- +goose Up
-- 企业客服设置增加客户身份密钥，联系人增加企业用户编号，客服处理周期增加访客上下文。
ALTER TABLE customer_service_settings
    ADD COLUMN customer_identity_secret text;

COMMENT ON COLUMN customer_service_settings.customer_identity_secret IS '网站登录用户签名身份的 HMAC 密钥，base64url 字符串；未生成时为空，此时不接受签名身份';

ALTER TABLE contacts
    ADD COLUMN external_user_id text;

COMMENT ON COLUMN contacts.external_user_id IS '企业用户编号，取自验签通过的签名身份 sub；未验证身份的联系人为空';

CREATE UNIQUE INDEX contacts_organization_external_user_unique
    ON contacts (organization_id, external_user_id)
    WHERE external_user_id IS NOT NULL;

ALTER TABLE service_sessions
    ADD COLUMN visitor_context jsonb;

COMMENT ON COLUMN service_sessions.visitor_context IS '网站访客上下文：来源页、当前页、设备、语言、时区与地区，随该周期的访客消息更新；非网站渠道为空';

-- +goose Down
ALTER TABLE service_sessions
    DROP COLUMN visitor_context;

DROP INDEX contacts_organization_external_user_unique;

ALTER TABLE contacts
    DROP COLUMN external_user_id;

ALTER TABLE customer_service_settings
    DROP COLUMN customer_identity_secret;
