-- +goose Up
ALTER TABLE knowledge_documents
    ADD COLUMN source_kind varchar(16) NOT NULL DEFAULT 'file',
    ADD COLUMN title text NOT NULL DEFAULT '',
    ADD COLUMN source_url text NOT NULL DEFAULT '',
    ALTER COLUMN file_id DROP NOT NULL;

DROP INDEX knowledge_documents_file_unique;
CREATE UNIQUE INDEX knowledge_documents_file_unique ON knowledge_documents (file_id) WHERE file_id IS NOT NULL;
CREATE UNIQUE INDEX knowledge_documents_source_url_unique ON knowledge_documents (knowledge_base_id, source_url) WHERE source_kind = 'web';

COMMENT ON COLUMN knowledge_documents.source_kind IS '内容来源：file 上传原件、text 在线编写、web 网页导入';
COMMENT ON COLUMN knowledge_documents.title IS '在线文档与网页文档的名称';
COMMENT ON COLUMN knowledge_documents.source_url IS '网页文档的页面地址';
COMMENT ON COLUMN knowledge_documents.file_id IS '原件编号及上传重试幂等键，非上传来源为空';

-- +goose Down
DELETE FROM knowledge_segments WHERE source_type = 'document' AND source_id IN (SELECT id FROM knowledge_documents WHERE source_kind <> 'file');
DELETE FROM knowledge_documents WHERE source_kind <> 'file';

DROP INDEX knowledge_documents_source_url_unique;
DROP INDEX knowledge_documents_file_unique;
CREATE UNIQUE INDEX knowledge_documents_file_unique ON knowledge_documents (file_id);

ALTER TABLE knowledge_documents
    ALTER COLUMN file_id SET NOT NULL,
    DROP COLUMN source_url,
    DROP COLUMN title,
    DROP COLUMN source_kind;

COMMENT ON COLUMN knowledge_documents.file_id IS '原件编号及上传重试幂等键';
