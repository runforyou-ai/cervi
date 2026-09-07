-- 管理员使用 psql -v cervi_role=实际应用账号 -d 目标数据库 -f prepare-admin.sql 执行。
\set ON_ERROR_STOP on
BEGIN;
SELECT pg_advisory_xact_lock(hashtextextended('cervi:database:prepare', 0));
\ir prepare.sql
GRANT USAGE ON SCHEMA public TO :"cervi_role";
GRANT USAGE, CREATE ON SCHEMA haystack TO :"cervi_role";
COMMIT;
