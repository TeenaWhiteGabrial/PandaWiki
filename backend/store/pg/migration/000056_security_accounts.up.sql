ALTER TABLE api_tokens ADD COLUMN IF NOT EXISTS token_hint TEXT NOT NULL DEFAULT '';
-- Migrate existing credentials without invalidating clients. PostgreSQL's
-- built-in sha256(bytea) needs no pgcrypto extension or superuser permission.
UPDATE api_tokens
SET token_hint = left(token, 6) || '…' || right(token, 4),
    token = 'sha256:' || encode(sha256(convert_to(token, 'UTF8')), 'hex')
WHERE token NOT LIKE 'sha256:%';

ALTER TABLE users ADD COLUMN IF NOT EXISTS password_change_required BOOLEAN NOT NULL DEFAULT FALSE;
-- Existing password accounts may use a shared initial password; require rotation.
UPDATE users SET password_change_required = TRUE WHERE password <> '';
CREATE TABLE IF NOT EXISTS account_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS account_sessions_user_idx ON account_sessions(user_id);
CREATE INDEX IF NOT EXISTS account_sessions_expiry_idx ON account_sessions(expires_at);
