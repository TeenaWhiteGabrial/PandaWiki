DROP INDEX IF EXISTS idx_runtime_logs_source_time;
ALTER TABLE runtime_logs DROP COLUMN IF EXISTS source;
