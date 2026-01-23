BEGIN;

CREATE TABLE IF NOT EXISTS oauth_accounts (
    provider TEXT NOT NULL,
    subject TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    display_name TEXT NOT NULL,
    linked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (provider, subject)
);

CREATE INDEX IF NOT EXISTS oauth_accounts_user_idx ON oauth_accounts (user_id);

COMMIT;
