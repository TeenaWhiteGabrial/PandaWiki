ALTER TABLE node_release_backup DROP COLUMN IF EXISTS editor_version;
ALTER TABLE node_release_backup DROP COLUMN IF EXISTS editor_format;
ALTER TABLE node_release_backup DROP COLUMN IF EXISTS editor_blocks;
ALTER TABLE node_release_backup DROP COLUMN IF EXISTS editor_content;

ALTER TABLE node_releases DROP COLUMN IF EXISTS editor_version;
ALTER TABLE node_releases DROP COLUMN IF EXISTS editor_format;
ALTER TABLE node_releases DROP COLUMN IF EXISTS editor_blocks;
ALTER TABLE node_releases DROP COLUMN IF EXISTS editor_content;
