CREATE EXTENSION IF NOT EXISTS pgcrypto;

ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS hashed_token text;

UPDATE auth_sessions
SET hashed_token = encode(digest(token::bytea, 'sha256'), 'hex')
WHERE hashed_token IS NULL;

ALTER TABLE auth_sessions ALTER COLUMN hashed_token SET NOT NULL;

ALTER TABLE auth_sessions DROP CONSTRAINT IF EXISTS auth_sessions_pkey;
ALTER TABLE auth_sessions ADD CONSTRAINT auth_sessions_pkey PRIMARY KEY (hashed_token);

CREATE UNIQUE INDEX IF NOT EXISTS auth_sessions_hashed_token_idx ON auth_sessions (hashed_token);

UPDATE auth_sessions SET token = hashed_token;
