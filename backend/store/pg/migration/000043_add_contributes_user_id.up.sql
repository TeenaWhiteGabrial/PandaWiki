ALTER TABLE contributes ADD COLUMN IF NOT EXISTS user_id text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_contributes_user_id ON contributes(user_id);
