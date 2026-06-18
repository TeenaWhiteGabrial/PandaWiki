ALTER TABLE node_releases ADD COLUMN IF NOT EXISTS editor_content jsonb;
ALTER TABLE node_releases ADD COLUMN IF NOT EXISTS editor_blocks jsonb;
ALTER TABLE node_releases ADD COLUMN IF NOT EXISTS editor_format text NOT NULL DEFAULT '';
ALTER TABLE node_releases ADD COLUMN IF NOT EXISTS editor_version integer NOT NULL DEFAULT 0;

ALTER TABLE node_release_backup ADD COLUMN IF NOT EXISTS editor_content jsonb;
ALTER TABLE node_release_backup ADD COLUMN IF NOT EXISTS editor_blocks jsonb;
ALTER TABLE node_release_backup ADD COLUMN IF NOT EXISTS editor_format text NOT NULL DEFAULT '';
ALTER TABLE node_release_backup ADD COLUMN IF NOT EXISTS editor_version integer NOT NULL DEFAULT 0;
