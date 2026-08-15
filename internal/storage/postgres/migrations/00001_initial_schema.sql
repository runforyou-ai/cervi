-- +goose Up
-- This baseline intentionally contains no domain tables. Goose records it in
-- goose_db_version, establishing versioned schema management for future work.
SELECT 1;

-- +goose Down
SELECT 1;
