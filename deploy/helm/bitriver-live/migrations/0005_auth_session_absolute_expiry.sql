ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS absolute_expires_at TIMESTAMPTZ;

UPDATE auth_sessions
SET absolute_expires_at = expires_at
WHERE absolute_expires_at IS NULL;

ALTER TABLE auth_sessions ALTER COLUMN absolute_expires_at SET NOT NULL;

CREATE INDEX IF NOT EXISTS auth_sessions_absolute_expires_at_idx ON auth_sessions (absolute_expires_at);
