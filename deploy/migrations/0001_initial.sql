-- 0001_initial.sql
--
-- Establishes the relational schema that mirrors the entities defined in
-- internal/models. The layout keeps identifiers as text so existing snowflake
-- and ULID generators used by the JSON repository continue to work without
-- translation when importing data into Postgres.

BEGIN;

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    roles TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    password_hash TEXT,
    self_signup BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS profiles (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    bio TEXT NOT NULL DEFAULT '',
    avatar_url TEXT,
    banner_url TEXT,
    featured_channel_id TEXT,
    top_friends TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    donation_addresses JSONB NOT NULL DEFAULT '[]'::JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS channels (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stream_key TEXT NOT NULL,
    title TEXT NOT NULL,
    category TEXT,
    tags TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    live_state TEXT NOT NULL,
    current_session_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS follows (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    followed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, channel_id)
);

CREATE TABLE IF NOT EXISTS stream_sessions (
    id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    renditions TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    peak_concurrent INTEGER NOT NULL DEFAULT 0,
    origin_url TEXT,
    playback_url TEXT,
    ingest_endpoints TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    ingest_job_ids TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[]
);

CREATE TABLE IF NOT EXISTS stream_session_manifests (
    session_id TEXT NOT NULL REFERENCES stream_sessions(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    manifest_url TEXT NOT NULL,
    bitrate INTEGER,
    PRIMARY KEY (session_id, name)
);

CREATE TABLE IF NOT EXISTS recordings (
    id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL REFERENCES stream_sessions(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    duration_seconds INTEGER NOT NULL,
    playback_base_url TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::JSONB,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    retain_until TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS recording_renditions (
    recording_id TEXT NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    manifest_url TEXT NOT NULL,
    bitrate INTEGER,
    PRIMARY KEY (recording_id, name)
);

CREATE TABLE IF NOT EXISTS recording_thumbnails (
    id TEXT PRIMARY KEY,
    recording_id TEXT NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    width INTEGER,
    height INTEGER,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS clip_exports (
    id TEXT PRIMARY KEY,
    recording_id TEXT NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL REFERENCES stream_sessions(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    start_seconds INTEGER NOT NULL,
    end_seconds INTEGER NOT NULL,
    status TEXT NOT NULL,
    playback_url TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    storage_object TEXT
);

CREATE TABLE IF NOT EXISTS chat_messages (
    id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS chat_bans (
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL DEFAULT '',
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (channel_id, user_id)
);

CREATE INDEX IF NOT EXISTS chat_bans_channel_idx ON chat_bans (channel_id);

CREATE TABLE IF NOT EXISTS chat_timeouts (
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL DEFAULT '',
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (channel_id, user_id)
);

CREATE INDEX IF NOT EXISTS chat_timeouts_channel_idx ON chat_timeouts (channel_id, expires_at);

CREATE TABLE IF NOT EXISTS chat_reports (
    id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    reporter_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason TEXT NOT NULL,
    message_id TEXT REFERENCES chat_messages(id) ON DELETE SET NULL,
    evidence_url TEXT,
    status TEXT NOT NULL DEFAULT 'open',
    resolution TEXT,
    resolver_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS chat_reports_channel_status_idx ON chat_reports (channel_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS chat_reports_target_idx ON chat_reports (target_id);

CREATE TABLE IF NOT EXISTS tips (
    id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    from_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount NUMERIC(20, 8) NOT NULL,
    currency TEXT NOT NULL,
    provider TEXT NOT NULL,
    reference TEXT NOT NULL,
    wallet_address TEXT,
    message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS tips_reference_unique ON tips (provider, reference);
CREATE INDEX IF NOT EXISTS tips_channel_created_idx ON tips (channel_id, created_at DESC);

CREATE TABLE IF NOT EXISTS subscriptions (
    id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tier TEXT NOT NULL,
    provider TEXT NOT NULL,
    reference TEXT NOT NULL,
    amount NUMERIC(20, 8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    auto_renew BOOLEAN NOT NULL DEFAULT FALSE,
    status TEXT NOT NULL DEFAULT 'active',
    cancelled_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    cancelled_reason TEXT,
    cancelled_at TIMESTAMPTZ,
    external_reference TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS subscriptions_reference_unique ON subscriptions (provider, reference);
CREATE INDEX IF NOT EXISTS subscriptions_channel_status_idx ON subscriptions (channel_id, status, started_at DESC);
CREATE INDEX IF NOT EXISTS subscriptions_user_channel_idx ON subscriptions (user_id, channel_id);

COMMIT;
