ALTER TABLE runtime_logs ADD COLUMN IF NOT EXISTS source varchar(512) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_runtime_logs_source_time ON runtime_logs(source, event_time DESC, id DESC);
