-- +goose Up
ALTER TABLE public.knowledge_segments ADD COLUMN context text NOT NULL DEFAULT '';
ALTER TABLE public.knowledge_segments ALTER COLUMN context DROP DEFAULT;
COMMENT ON COLUMN public.knowledge_segments.context IS '分段起点所属的标题路径和表头，与正文共同用于向量、词法词元和重排';

-- +goose Down
ALTER TABLE public.knowledge_segments DROP COLUMN context;
