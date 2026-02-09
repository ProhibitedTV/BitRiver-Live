-- GENERATED FILE: DO NOT EDIT DIRECTLY
-- Canonical source: deploy/migrations/0002_auth_sessions.sql
-- Regenerate with: ./scripts/sync-helm-deploy-assets.sh

CREATE TABLE IF NOT EXISTS auth_sessions (
    token TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS auth_sessions_expires_at_idx
    ON auth_sessions (expires_at);
