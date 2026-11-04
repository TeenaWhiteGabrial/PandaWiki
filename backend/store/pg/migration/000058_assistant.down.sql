DROP TABLE IF EXISTS assistant_objects, assistant_artifacts, assistant_run_attempts, assistant_runs, assistant_preferences, assistant_templates, assistant_uploads;

ALTER TABLE conversation_messages DROP COLUMN IF EXISTS assistant_sources;
