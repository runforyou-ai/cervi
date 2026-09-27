-- +goose Up
-- 联系人字段与标签增加 AI 填写说明和添加条件，AI 写入的取值与标签记录来源周期。
ALTER TABLE contact_fields
    ADD COLUMN ai_instruction text NOT NULL DEFAULT '';

COMMENT ON COLUMN contact_fields.ai_instruction IS 'AI 填写说明；为空表示 AI 不填写该字段';

ALTER TABLE contact_tags
    ADD COLUMN ai_instruction text NOT NULL DEFAULT '';

COMMENT ON COLUMN contact_tags.ai_instruction IS 'AI 添加条件；为空表示只能由客服添加';

ALTER TABLE contact_field_values
    ADD COLUMN source_service_session_id uuid,
    ADD COLUMN source_session_closed_at timestamptz;

COMMENT ON COLUMN contact_field_values.source IS '取值来源：member 客服填写，ai AI 根据对话填写';
COMMENT ON COLUMN contact_field_values.source_service_session_id IS '来源为 AI 时抽取所依据的客服周期编号';
COMMENT ON COLUMN contact_field_values.source_session_closed_at IS '来源为 AI 时抽取所依据的那次周期关闭时间，AI 只用关闭更晚的周期覆盖取值';

ALTER TABLE contact_tag_assignments
    ADD COLUMN source_service_session_id uuid;

COMMENT ON COLUMN contact_tag_assignments.source IS '添加来源：member 客服添加，ai AI 根据对话添加';
COMMENT ON COLUMN contact_tag_assignments.source_service_session_id IS '来源为 AI 时判断所依据的客服周期编号';

-- +goose Down
COMMENT ON COLUMN contact_tag_assignments.source IS '添加来源：member 客服添加';
ALTER TABLE contact_tag_assignments
    DROP COLUMN source_service_session_id;

COMMENT ON COLUMN contact_field_values.source IS '取值来源：member 客服填写';
ALTER TABLE contact_field_values
    DROP COLUMN source_service_session_id,
    DROP COLUMN source_session_closed_at;

ALTER TABLE contact_tags
    DROP COLUMN ai_instruction;

ALTER TABLE contact_fields
    DROP COLUMN ai_instruction;
