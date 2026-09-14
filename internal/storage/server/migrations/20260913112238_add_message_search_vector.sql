-- +goose Up
ALTER TABLE messages ADD COLUMN search_vector tsvector NOT NULL DEFAULT ''::tsvector;
ALTER TABLE messages ALTER COLUMN search_vector SET STATISTICS 3000;
CREATE INDEX messages_search_vector ON messages USING gin (search_vector);
CREATE INDEX messages_organization_originated ON messages (organization_id, originated_at, id);
COMMENT ON COLUMN messages.search_vector IS '消息检索词元：正文与附件文件名的单字、字母数字片段和汉字全拼读音';

-- +goose Down
DROP INDEX messages_organization_originated;
DROP INDEX messages_search_vector;
ALTER TABLE messages DROP COLUMN search_vector;
