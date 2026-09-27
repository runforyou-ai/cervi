-- +goose Up
-- 联系人字段取值和标签新增网站来源。
COMMENT ON COLUMN contact_field_values.source IS '取值来源：member 客服填写，ai AI 根据对话填写，website 网站签名身份同步';
COMMENT ON COLUMN contact_tag_assignments.source IS '添加来源：member 客服添加，ai AI 根据对话添加，website 网站签名身份同步';

-- +goose Down
COMMENT ON COLUMN contact_field_values.source IS '取值来源：member 客服填写，ai AI 根据对话填写';
COMMENT ON COLUMN contact_tag_assignments.source IS '添加来源：member 客服添加，ai AI 根据对话添加';
