-- 在当前数据库安装缺失的扩展和检索 schema，不升级或移动已有扩展。
DO $$
DECLARE
    extension_name text;
BEGIN
    FOREACH extension_name IN ARRAY ARRAY['vector', 'pg_trgm'] LOOP
        IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = extension_name) THEN
            IF NOT EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = extension_name) THEN
                RAISE EXCEPTION '数据库 % 缺少 % 扩展安装文件，请更换 PostgreSQL 镜像或安装扩展包', current_database(), extension_name;
            END IF;
            EXECUTE format('CREATE EXTENSION %I WITH SCHEMA public', extension_name);
        END IF;
    END LOOP;
    IF NOT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'haystack') THEN
        CREATE SCHEMA haystack;
    END IF;
END
$$;
