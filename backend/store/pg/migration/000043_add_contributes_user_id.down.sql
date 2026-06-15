DROP INDEX IF EXISTS idx_contributes_user_id;

ALTER TABLE contributes DROP COLUMN IF EXISTS user_id;
